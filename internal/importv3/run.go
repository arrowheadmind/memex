package importv3

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// Options of an import run.
type Options struct {
	Sources []string // SRC[@REV], the first is the primary
	Out     string   // DIR, must not exist
	Leaks   []string
	Today   time.Time
	Home    string
	Version string
	Timeout time.Duration
	Stdout  io.Writer
	Stderr  io.Writer // progress notes, such as the dating cost; nil for none
}

// Importer holds one run.
type Importer struct {
	opts        Options
	today       string
	home        string
	srcs        []*Source
	primary     *Source
	files       []*fileRec
	units       []*Unit
	report      *Report
	ids         map[string]*Unit
	byKey       map[*Source]map[string][]*Unit
	aliasOwner  map[string]*Unit
	unresolved  map[string]int
	brainRepo   map[*Source]string
	attach      []attachment
	attachOwner []*Unit // per attachment, the entry whose files/ directory holds it
	agentMD     string
	prefixes    []string
	fullRe      *regexp.Regexp
	wrappedRe   *regexp.Regexp
	datingCost  time.Duration
	mu          sync.Mutex // guards report fields written by the dating workers
}

var brainRepoRe = regexp.MustCompile(`(?m)^- \*\*BRAIN_REPO\*\*: *(.+)$`)
var logPrefixRe = regexp.MustCompile(`^\d{8}T\d{6}Z-[^-]+-`)

type attachment struct {
	Src  *Source
	Path string // source path
	Dest string // relative to DIR
}

// Failure is an import that ran but must not be used: accounting or validation problems.
type Failure struct{ Problems []string }

func (f *Failure) Error() string { return fmt.Sprintf("import has %d problems", len(f.Problems)) }

// Run performs the import (SPEC §13) and returns the report. A *Failure error means the
// output was written but exits 1.
func Run(o Options) (*Report, error) {
	im := &Importer{opts: o, today: o.Today.Format(time.DateOnly), home: o.Home,
		report:     &Report{Today: o.Today.Format(time.DateOnly)},
		unresolved: map[string]int{}, brainRepo: map[*Source]string{}, aliasOwner: map[string]*Unit{}}
	if _, err := os.Lstat(o.Out); err == nil {
		return nil, fmt.Errorf("%s exists; --out must name a new directory", o.Out)
	}
	for i, arg := range o.Sources {
		s, err := OpenSource(arg, i == 0, o.Timeout)
		if err != nil {
			return nil, err
		}
		im.srcs = append(im.srcs, s)
		im.report.Sources = append(im.report.Sources, srcRec{Rev: s.Rev, Commit: s.Commit, Basename: s.Basename, Primary: s.Primary})
	}
	im.primary = im.srcs[0]
	for _, s := range im.srcs {
		if err := im.walk(s); err != nil {
			return nil, err
		}
	}
	if err := im.leaks(); err != nil {
		return nil, err
	}
	im.buildResolver()
	for _, s := range im.srcs {
		if s.Has(".brain/INDEX.md") {
			f, err := im.file(s, ".brain/INDEX.md")
			if err != nil {
				return nil, err
			}
			im.extractIndex(f)
		}
	}
	if err := im.resolveDates(); err != nil {
		return nil, err
	}
	im.resolveStatuses()
	im.applyRepos()
	im.dropDuplicates()
	if err := im.remainders(); err != nil {
		return nil, err
	}
	if err := im.attachmentsEntries(); err != nil {
		return nil, err
	}
	im.assignIDs()
	im.resolveAliases()
	for _, u := range im.units {
		for _, a := range u.Aliases {
			im.aliasOwner[a] = u
		}
	}
	im.resolveSuperseded()
	for _, u := range im.units {
		if u.Kind == "dropped" {
			continue
		}
		u.Body = im.rewriteLinks(u, strings.Join(u.BodyLines, "\n"))
	}
	im.report.Links.Unresolved = im.unresolvedList()
	sort.Strings(im.report.DatedByBlame)
	if o.Stderr != nil {
		fmt.Fprintf(o.Stderr, "memex: dated line units in %.1fs\n", im.datingCost.Seconds())
	}
	im.account()
	if err := im.write(); err != nil {
		return nil, err
	}
	if err := im.validate(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(o.Out, "import-report.json"), im.report.JSON(), 0o644); err != nil {
		return nil, err
	}
	if len(im.report.Problems) > 0 {
		return im.report, &Failure{Problems: im.report.Problems}
	}
	return im.report, nil
}

// file returns the record of a recognized source file, reading it once.
func (im *Importer) file(s *Source, p string) (*fileRec, error) {
	for _, f := range im.files {
		if f.Src == s && f.Path == p {
			return f, nil
		}
	}
	f, err := newFileRec(s, p)
	if err != nil {
		return nil, err
	}
	im.files = append(im.files, f)
	return f, nil
}

var notKnowledgeNames = map[string]bool{".gitkeep": true, ".gitignore": true, ".gitattributes": true, ".envrc": true, "CLAUDE.md": true, "AGENTS.md": true, "GEMINI.md": true}

// walk classifies every tracked file of a source (SPEC §13.2) and extracts its units in the
// §13.3 order.
func (im *Importer) walk(s *Source) error {
	kind := func(p string) string {
		base := path.Base(p)
		switch {
		case notKnowledgeNames[base]:
			return "not"
		case p == ".brain/README.md", p == ".brain/plans/README.md":
			return "not"
		case strings.HasPrefix(p, ".brain/templates/"), strings.HasPrefix(p, ".brain/scripts/"), strings.HasPrefix(p, ".githooks/"), strings.HasPrefix(p, "scripts/"), strings.HasPrefix(p, "templates/"):
			return "not"
		case strings.HasPrefix(p, ".brain/investigations/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 2:
			return "investigation"
		case strings.HasPrefix(p, ".brain/decisions/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 2:
			return "decision"
		case strings.HasPrefix(p, ".brain/runbooks/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 2:
			return "runbook"
		case strings.HasPrefix(p, ".brain/plans/prompts/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 3:
			return "prompt"
		case strings.HasPrefix(p, ".brain/plans/reports/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 3:
			return "report"
		case strings.HasPrefix(p, ".brain/plans/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 2:
			return "plan"
		case strings.HasPrefix(p, ".brain/plans/"), strings.HasPrefix(p, ".brain/dispatch-logs/"):
			return "attachment"
		case p == ".brain/known-mistakes.md":
			return "known-mistakes"
		case p == ".brain/constraints.md":
			return "constraints"
		case p == ".brain/INDEX.md":
			return "index"
		case p == ".brain/AGENT.md":
			return "agent"
		case p == ".brain/backlog.md":
			return "backlog"
		case s.Primary && p == "docs/backlog.md":
			return "backlog"
		case s.Primary && strings.HasPrefix(p, "docs/runbooks/") && strings.HasSuffix(p, ".md") && strings.Count(p, "/") == 2:
			return "docs-runbook"
		case s.Primary && strings.HasPrefix(p, "docs/") && strings.HasSuffix(p, ".md"):
			return "doc"
		}
		return "not"
	}
	byKind := map[string][]string{}
	for _, p := range s.Files() {
		k := kind(p)
		byKind[k] = append(byKind[k], p)
	}
	for _, p := range byKind["not"] {
		im.report.NotKnowledge = append(im.report.NotKnowledge, s.Basename+":"+p)
	}
	if p := ".brain/AGENT.md"; s.Has(p) {
		text, err := s.Read(p)
		if err != nil {
			return err
		}
		if m := brainRepoRe.FindStringSubmatch(text); m != nil {
			im.brainRepo[s] = tagify(m[1])
		}
		if s.Primary {
			f, err := im.file(s, p)
			if err != nil {
				return err
			}
			u := &Unit{Kind: "agent", Title: "AGENT.md"}
			var body []int
			for i := range f.Lines {
				body = append(body, i)
			}
			im.add(f, u, body, nil, nil)
			im.agentMD = text
		} else {
			im.report.NotKnowledge = append(im.report.NotKnowledge, s.Basename+":"+p)
		}
	}
	order := []struct{ kind, unit, typ string }{
		{"investigation", "investigation", "journal"}, {"decision", "decision", "decision"},
		{"runbook", "runbook", "runbook"}, {"plan", "plan", "plan"}, {"prompt", "prompt", "plan"},
		{"report", "report", "plan"}, {"doc", "doc", "reference"}, {"docs-runbook", "runbook", "runbook"},
	}
	for _, o := range order {
		for _, p := range byKind[o.kind] {
			f, err := im.file(s, p)
			if err != nil {
				return err
			}
			u := im.extractFile(f, o.unit, o.typ)
			if o.kind == "prompt" {
				u.PlanKind = "prompt"
			}
			if o.kind == "report" {
				u.PlanKind = "report"
			}
		}
	}
	for _, p := range byKind["known-mistakes"] {
		f, err := im.file(s, p)
		if err != nil {
			return err
		}
		im.extractRules(f, false)
	}
	for _, p := range byKind["constraints"] {
		f, err := im.file(s, p)
		if err != nil {
			return err
		}
		im.extractRules(f, true)
	}
	for _, p := range byKind["backlog"] {
		f, err := im.file(s, p)
		if err != nil {
			return err
		}
		im.extractBacklog(f)
	}
	for _, p := range byKind["attachment"] {
		im.attach = append(im.attach, attachment{Src: s, Path: p})
	}
	return nil
}

// leaks makes a raw journal entry per --leak file, as capture --from would (SPEC §11.2).
func (im *Importer) leaks() error {
	for _, p := range im.opts.Leaks {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		shown := p // written as given, except that the home directory becomes ~
		if im.home != "" && strings.HasPrefix(p, im.home+"/") {
			shown = "~/" + strings.TrimPrefix(p, im.home+"/")
		}
		text := string(data)
		title := ""
		for _, l := range splitLines(text) {
			if lvl, t := heading(l); lvl > 0 {
				title = t
				break
			}
		}
		summary := "Imported from " + shown
		if title == "" {
			name, desc := yamlBlock(splitLines(text))
			title = name
			if desc != "" {
				summary = desc
			}
		}
		if title == "" {
			title = filepath.Base(p)
		}
		u := &Unit{Kind: "leak", Type: "journal", Title: cutTitle(stripMarkup(title)), Status: "raw",
			Summary: summary, Aliases: []string{shown}, Date: im.today,
			Src: im.primary, File: shown}
		u.BodyLines = []string{fmt.Sprintf("Imported from `%s` (%s).", shown, im.today), "", strings.TrimRight(text, "\n")}
		u.Order = len(im.units)
		im.units = append(im.units, u)
		im.report.Leaks = append(im.report.Leaks, shown)
	}
	return nil
}

// applyRepos fills repos from the investigation header or BRAIN_REPO (SPEC §13.3).
func (im *Importer) applyRepos() {
	for _, u := range im.units {
		var repos []string
		for _, r := range u.Repos {
			if t := tagify(r); t != "" && !im.emptyRepo(t) {
				repos = append(repos, t)
			}
		}
		if len(repos) == 0 && u.Kind != "leak" {
			if r := im.brainRepo[u.Src]; r != "" && !im.emptyRepo(r) {
				repos = []string{r}
			}
		}
		u.Repos = repos
		var tags []string
		for _, t := range u.Tags {
			n := tagify(t)
			if n == "" || contextNames[n] { // a brainctl context name is a placeholder, not a tag
				continue
			}
			tags = append(tags, n)
		}
		u.Tags = tags
	}
}

// remainders makes one reference entry per source file with remainder lines (SPEC §13.6).
func (im *Importer) remainders() error {
	for _, f := range im.files {
		var idx []int
		for i := range f.Lines {
			if f.Attr[i] == attrRemainder && strings.TrimSpace(f.Lines[i]) != "" {
				idx = append(idx, i)
			} else if f.Attr[i] == attrRemainder {
				f.Attr[i] = attrStructural
			}
		}
		if len(idx) == 0 {
			continue
		}
		name := f.Path
		if !f.Src.Primary {
			name = f.Src.Basename + "/" + f.Path
		}
		u := &Unit{Kind: "remainder", Type: "reference", Title: cutTitle(name + " (import remainder)"), Status: "active"}
		first, err := f.Src.FirstDay(f.Path)
		if err != nil {
			return err
		}
		last, err := f.Src.LastDay(f.Path)
		if err != nil {
			return err
		}
		u.Date, u.Verified = first, last
		im.add(f, u, idx, nil, nil)
		for _, i := range idx {
			f.Attr[i] = attrRemainder
		}
	}
	return nil
}

// attachmentsEntries assigns an owner to every attachment and creates the per-source
// attachments entry where needed (SPEC §13.3).
func (im *Importer) attachmentsEntries() error {
	holder := map[*Source]*Unit{}
	for i := range im.attach {
		a := &im.attach[i]
		rel := strings.TrimPrefix(a.Path, ".brain/plans/")
		isLog := strings.HasPrefix(a.Path, ".brain/dispatch-logs/")
		if isLog {
			rel = strings.TrimPrefix(a.Path, ".brain/dispatch-logs/")
		}
		dir := rel
		if k := strings.Index(rel, "/"); k >= 0 {
			dir = rel[:k]
		} else {
			dir = stem(rel)
		}
		if isLog {
			dir = logPrefixRe.ReplaceAllString(stem(rel), "")
		}
		var owner *Unit
		best := -1
		for _, u := range im.units {
			if u.Src != a.Src || u.Kind != "plan" { // a dropped duplicate is no longer a plan
				continue
			}
			if u.Stem == dir || (strings.HasPrefix(dir, u.Stem) && len(u.Stem) < len(dir)) {
				if len(u.Stem) > best {
					best = len(u.Stem)
					owner = u
				}
			}
		}
		if owner == nil {
			owner = holder[a.Src]
			if owner == nil {
				day, err := a.Src.CommitDay()
				if err != nil {
					return err
				}
				owner = &Unit{Kind: "attachments", Type: "reference", Title: "Import attachments from " + a.Src.Basename,
					Status: "active", Date: day, Verified: day, Src: a.Src, File: a.Src.Basename, Order: len(im.units), Secondary: !a.Src.Primary}
				im.units = append(im.units, owner)
				holder[a.Src] = owner
			}
		}
		a.Dest = rel
		if isLog {
			a.Dest = "dispatch-logs/" + rel
		}
		im.attachOwner = append(im.attachOwner, owner)
	}
	return nil
}

// typeDir returns the directory of a type.
func typeDir(typ string) string {
	for _, t := range spec.Types {
		if t.Name == typ {
			return t.Directory
		}
	}
	panic("unknown type " + typ)
}

// write creates DIR as init does and writes every entry, attachment and AGENT.md.
func (im *Importer) write() error {
	g := gitx.New(im.opts.Out, im.opts.Timeout)
	if _, err := store.CreateLayout(im.opts.Out, im.primary.Basename, g); err != nil {
		return err
	}
	// attachments entries' bodies list the files they own
	owned := map[*Unit][]string{}
	for i, a := range im.attach {
		o := im.attachOwner[i]
		owned[o] = append(owned[o], "- files/"+o.ID+"/"+a.Dest)
	}
	for u, list := range owned {
		if u.Kind == "attachments" {
			sort.Strings(list)
			u.BodyLines = list
			u.Body = strings.Join(list, "\n")
		}
	}
	for _, u := range im.units {
		if u.Kind == "dropped" || u.Kind == "agent" {
			continue
		}
		body := u.Body
		if u.IndexPrefix != "" {
			body = u.IndexPrefix + "\n\n" + body
		}
		typ := spec.LookupType(u.Type)
		body = entry.CanonicalBody(typ, entry.Normalize(body))
		created, err := time.Parse(time.DateOnly, u.Date)
		if err != nil {
			return fmt.Errorf("%s: bad date %q", u.ID, u.Date)
		}
		fields := entry.Fields{"title": u.Title, "status": u.Status, "created": created}
		if u.Summary != "" {
			fields["summary"] = u.Summary
		}
		if len(u.Tags) > 0 {
			fields["tags"] = sortedTags(u.Tags)
		}
		if len(u.Repos) > 0 {
			fields["repos"] = sortedTags(u.Repos)
		}
		if u.Always {
			fields["always"] = true
		}
		if u.PlanKind != "" {
			fields["kind"] = u.PlanKind
		}
		if u.SupersededBy != "" {
			fields["superseded_by"] = u.SupersededBy
		}
		if u.Type == "reference" {
			v, err := time.Parse(time.DateOnly, u.Verified)
			if err != nil {
				return fmt.Errorf("%s: bad verified date %q", u.ID, u.Verified)
			}
			fields["verified"] = v
		}
		if len(u.Aliases) > 0 {
			fields["aliases"] = u.Aliases
		}
		if u.Type == "journal" {
			fields["body_sha256"] = entry.BodySHA256(body)
		}
		rel := typeDir(u.Type) + "/" + u.ID + ".md"
		if err := os.WriteFile(filepath.Join(im.opts.Out, rel), []byte(entry.Render(fields, body)), 0o644); err != nil {
			return err
		}
		lines := [2]int{}
		if len(u.Lines) > 0 {
			sorted := append([]int(nil), u.Lines...)
			sort.Ints(sorted)
			lines = [2]int{sorted[0], sorted[len(sorted)-1]}
		}
		im.report.Entries = append(im.report.Entries, entryRec{ID: u.ID, Type: u.Type, Kind: u.Kind,
			Source: u.Src.Basename, Path: u.File, Status: u.Status, Lines: lines, Removed: u.Removed, Extra: u.Extra})
		if entry.ContentMeasure(body) < 80 {
			im.report.ShortBody = append(im.report.ShortBody, u.ID)
		}
	}
	for i, a := range im.attach {
		o := im.attachOwner[i]
		dest := filepath.Join("files", o.ID, filepath.FromSlash(a.Dest))
		data, err := a.Src.ReadBytes(a.Path)
		if err != nil {
			return err
		}
		full := filepath.Join(im.opts.Out, dest)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
		im.report.Attachments = append(im.report.Attachments, attachRec{Source: a.Src.Basename, Path: a.Path, Dest: filepath.ToSlash(dest)})
	}
	agent := store.AgentMD
	if im.agentMD != "" {
		agent = store.AgentMD + "\n" + im.agentMD
	}
	return os.WriteFile(filepath.Join(im.opts.Out, "AGENT.md"), []byte(agent), 0o644)
}

// validate runs check on the output: errors are import defects (SPEC §13.1).
func (im *Importer) validate() error {
	findings, err := check.Run(store.WorkTree{Root: im.opts.Out}, im.opts.Today, nil)
	if err != nil {
		return err
	}
	warns := map[string]int{}
	for _, f := range findings {
		if check.IsError(f) {
			im.report.Problems = append(im.report.Problems, fmt.Sprintf("check: %s: %s %s", f.Path, f.Code, f.Message))
		} else {
			warns[f.Code]++
		}
	}
	for code, n := range warns {
		im.report.Warnings = append(im.report.Warnings, countRec{Target: code, Count: n})
	}
	sort.Slice(im.report.Warnings, func(i, j int) bool { return im.report.Warnings[i].Target < im.report.Warnings[j].Target })
	return nil
}
