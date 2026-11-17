// Package check implements the validation of SPEC §7, except the workspace findings of §12.
package check

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/links"
	"github.com/arrowheadmind/memex/internal/slug"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// Finding is one line of `memex check` output.
type Finding = store.Finding

// IsError reports whether f is an error (E-) rather than a warning (W-).
func IsError(f Finding) bool { return strings.HasPrefix(f.Code, "E-") }

// Sort orders findings by path, then code, then message, by code point (SPEC §7).
func Sort(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := strings.Compare(a.Code, b.Code); c != 0 {
			return c
		}
		return strings.Compare(a.Message, b.Message)
	})
}

// Thresholds of §7.
const rawOldDays = 90

type parsed struct {
	store.File
	data     []byte
	body     string
	bodyLine int          // file line number of the body's first line
	fields   entry.Fields // nil when the frontmatter could not be read
}

type checker struct {
	tree     store.Tree
	today    time.Time
	findings []Finding
	files    []*parsed
	byID     map[string][]*parsed
	byAlias  map[string][]*parsed
	locked   map[string]bool
	// requireRepos is the manifest's require_repos (SPEC §4): W-NO-REPO is reported.
	requireRepos bool
	// staleDays is the manifest's stale_days (SPEC §4): W-OPEN-STALE's threshold, 0 for off.
	staleDays int64
	// format is the manifest's store format (SPEC §4): due and due_when need spec.DueFormat.
	format int64
	// targets records, for each entry path, the paths its links resolve to (for --staged).
	targets map[string][]string
}

func (c *checker) add(p, code, format string, args ...any) {
	c.findings = append(c.findings, Finding{Path: p, Code: code, Message: fmt.Sprintf(format, args...)})
}

func (c *checker) addImmutable(p, code, format string, args ...any) {
	c.findings = append(c.findings, Finding{Path: p, Code: code, Message: fmt.Sprintf(format, args...), Immutable: true})
}

// Run validates the store in tree. With staged non-nil, only findings about the staged paths
// and the entries their links resolve to are returned (SPEC §7, --staged). A manifest format
// this memex cannot read is a *store.FormatError.
func Run(tree store.Tree, today time.Time, staged []string) ([]Finding, error) {
	return RunHistory(tree, today, staged, nil)
}

// RunHistory is Run with W-STALE-SUMMARY, which needs the history of the task and plan
// files (SPEC §7, summary currency). With hist nil, that finding is not computed: the write
// path validates only errors, and reading history would cost it time for nothing.
func RunHistory(tree store.Tree, today time.Time, staged []string, hist *store.History) ([]Finding, error) {
	m, mf, err := store.ReadManifest(tree)
	if err != nil {
		return nil, err
	}
	layout, err := store.Enumerate(tree)
	if err != nil {
		return nil, err
	}
	c := &checker{tree: tree, today: today, byID: map[string][]*parsed{},
		byAlias: map[string][]*parsed{}, locked: map[string]bool{}, targets: map[string][]string{}}
	c.requireRepos = m.RequireRepos
	c.staleDays = m.StaleDays
	c.format = m.Format
	c.findings = append(c.findings, mf...)
	c.findings = append(c.findings, layout.Findings...)
	for _, f := range layout.Files {
		p, err := c.parse(f)
		if err != nil {
			return nil, err
		}
		c.files = append(c.files, p)
		c.byID[f.ID] = append(c.byID[f.ID], p)
	}
	c.checkIDs()
	c.checkAliases()
	for _, owner := range layout.Owners {
		if len(c.byID[owner]) == 0 {
			c.add("files/"+owner, "E-FILES-OWNER", "%s is not the id of an entry; attachments go in files/<id>/", q(owner))
		}
	}
	cited := map[string]bool{}
	for _, p := range c.files {
		for _, s := range p.fields.Strings("sources") {
			cited[s] = true
		}
	}
	for _, p := range c.files {
		if p.fields == nil {
			continue
		}
		if hist != nil {
			c.locked[p.Path] = lockedJournalBody(p, firstVersion(hist.Versions[p.Path]))
		}
		c.checkEntry(p, cited)
		if hist != nil {
			c.checkCurrency(p, hist.Versions[p.Path])
			c.checkStale(p, hist)
		}
	}
	c.checkCanonical()
	if staged != nil {
		c.findings = c.selectStaged(staged)
	}
	Sort(c.findings)
	return c.findings, nil
}

func firstVersion(vs [][]byte) []byte {
	if len(vs) == 0 {
		return nil
	}
	return vs[0]
}

// lockedJournalBody is the locked-body predicate of SPEC §7. head is nil unless HEAD
// holds a regular file at this path; p is the checked view (working tree or index).
func lockedJournalBody(p *parsed, head []byte) bool {
	if p.Type.Name != "journal" || head == nil || p.fields == nil {
		return false
	}
	sum := p.fields.String("body_sha256")
	if len(sum) != 64 {
		return false
	}
	for _, r := range sum {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	if sum != entry.BodySHA256(p.body) {
		return false
	}
	split, problem := entry.SplitFile(string(head))
	return problem == nil && split.Body == p.body
}

// parse reads one entry and records the encoding, frontmatter and field problems.
func (c *checker) parse(f store.File) (*parsed, error) {
	data, err := c.tree.ReadFile(f.Path)
	if err != nil {
		return nil, err
	}
	p := &parsed{File: f, data: data}
	if ps := entry.CheckEncoding(data); len(ps) > 0 {
		for _, pr := range ps {
			c.add(f.Path, pr.Code, "%s", pr.Message)
		}
		return p, nil
	}
	split, pr := entry.SplitFile(string(data))
	if pr != nil {
		c.add(f.Path, pr.Code, "%s", pr.Message)
		return p, nil
	}
	fields, ps := entry.ParseFields(split, f.Type, f.ID)
	ps = c.formatProblems(fields, ps)
	for _, pr := range ps {
		c.add(f.Path, pr.Code, "%s", pr.Message)
	}
	if fields == nil {
		return p, nil
	}
	p.fields = fields
	p.body = split.Body
	p.bodyLine = strings.Count(string(data[:len(data)-len(split.Body)]), "\n") + 1
	for _, a := range entry.SortedSet(fields.Strings("aliases")) {
		c.byAlias[a] = append(c.byAlias[a], p)
	}
	return p, nil
}

// formatProblems applies the rules that depend on the store format (SPEC §4, §5.3) to the
// fields ParseFields read and its problems. In a store before spec.DueFormat, due and
// due_when are E-FIELD-UNKNOWN with the command that allows them, and are removed from
// fields; from spec.DueFormat on, a control character in title, summary or due_when is
// E-FIELD-VALUE.
func (c *checker) formatProblems(fields entry.Fields, ps []entry.Problem) []entry.Problem {
	if fields == nil {
		return ps
	}
	if c.format < spec.DueFormat {
		for _, name := range spec.DueFields {
			if _, ok := fields[name]; !ok {
				continue
			}
			delete(fields, name)
			ps = slices.DeleteFunc(ps, func(pr entry.Problem) bool {
				return pr.Code == "E-FIELD-VALUE" && strings.HasPrefix(pr.Message, name+" ")
			})
			ps = append(ps, entry.Problem{Code: "E-FIELD-UNKNOWN", Message: fmt.Sprintf(
				"field %s needs store format %d; run memex migrate", q(name), spec.DueFormat)})
		}
		return ps
	}
	for _, name := range entry.ControlFreeFields {
		if v, ok := fields[name].(string); ok && entry.HasControl(v) {
			fields[name] = entry.Invalid{}
			ps = append(ps, entry.Problem{Code: "E-FIELD-VALUE", Message: name + " " + entry.ControlMessage})
		}
	}
	return ps
}

func paths(ps []*parsed, except *parsed) string {
	var out []string
	for _, p := range ps {
		if p != except {
			out = append(out, p.Path)
		}
	}
	return strings.Join(out, ", ")
}

func (c *checker) checkIDs() {
	for _, p := range c.files {
		if !slug.ValidID(p.ID) {
			c.add(p.Path, "E-ID", "filename is not a valid id: YYYY-MM-DD-slug, lowercase a-z, 0-9 and single hyphens, at most %d characters before .md", slug.MaxIDLength)
		}
		if same := c.byID[p.ID]; len(same) > 1 {
			c.add(p.Path, "E-DUP-ID", "id %s is also used by %s", q(p.ID), paths(same, p))
		}
	}
}

func (c *checker) checkAliases() {
	for alias, owners := range c.byAlias {
		for _, p := range owners {
			if len(owners) > 1 {
				c.add(p.Path, "E-ALIAS", "alias %s is also on %s", q(alias), paths(owners, p))
			}
			if ids := c.byID[alias]; len(ids) > 0 {
				c.add(p.Path, "E-ALIAS", "alias %s is the id of %s", q(alias), paths(ids, nil))
			}
		}
	}
}

// resolveRef checks a field reference (parent, sources, superseded_by): an id, never an alias.
func (c *checker) resolveRef(p *parsed, field, id string) {
	targets := c.byID[id]
	if len(targets) == 0 {
		msg := fmt.Sprintf("%s %s is not the id of an entry", field, q(id))
		if owners := c.byAlias[id]; len(owners) == 1 {
			msg += fmt.Sprintf("; it is an alias of %s, write its id %s", owners[0].Path, q(owners[0].ID))
		}
		c.add(p.Path, "E-REF", "%s", msg)
		return
	}
	if field == "parent" && !slices.ContainsFunc(targets, func(t *parsed) bool { return t.Type.Name == "plan" }) {
		c.add(p.Path, "E-REF", "parent %s is a %s, not a plan", q(id), targets[0].Type.Name)
	}
}

// brokenLink reports a [[TARGET]] shaped like a link that names no entry (W-BROKEN-LINK,
// SPEC §6 broken links), with the entry meant when exactly one is found.
func (c *checker) brokenLink(p *parsed, target string, line int) {
	meant := c.meant(target)
	switch {
	case c.locked[p.Path] && meant != "":
		c.addImmutable(p.Path, "W-BROKEN-LINK", "[[%s]] on line %d names no entry; it means [[%s]], but a journal body is immutable, so this is not counted as a warning", target, line, meant)
	case c.locked[p.Path]:
		c.addImmutable(p.Path, "W-BROKEN-LINK", "[[%s]] on line %d names no entry; a journal body is immutable, so this is not counted as a warning", target, line)
	case meant != "":
		c.add(p.Path, "W-BROKEN-LINK", "[[%s]] on line %d names no entry; link [[%s]] instead", target, line, meant)
	default:
		c.add(p.Path, "W-BROKEN-LINK", "[[%s]] on line %d names no entry; link the id of an entry, or put it in backticks if it is not a link", target, line)
	}
}

// meant returns the id of the entry a broken link means, or "" (SPEC §6 broken links): the
// target without a type directory prefix and a .md suffix is an id or a single alias; else,
// when that is an id written out beyond the length cap, the one id the cap could have made.
func (c *checker) meant(target string) string {
	t := target
	if dir, rest, ok := strings.Cut(t, "/"); ok && spec.LookupDirectory(dir) != nil {
		t = rest
	}
	t = strings.TrimSuffix(t, ".md")
	if len(c.byID[t]) > 0 {
		return t
	}
	if owners := c.byAlias[t]; len(owners) == 1 {
		return owners[0].ID
	}
	if len(t) <= slug.MaxIDLength || !slug.IDShaped(t) {
		return ""
	}
	prefix := strings.TrimRight(t[:111], "-") + "-"
	found := ""
	for id := range c.byID {
		if len(id) == len(prefix)+8 && strings.HasPrefix(id, prefix) && lowerHex(id[len(prefix):]) {
			if found != "" {
				return ""
			}
			found = id
		}
	}
	return found
}

func lowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

func (c *checker) checkEntry(p *parsed, cited map[string]bool) {
	f := p.fields
	if created, ok := f.Date("created"); ok && slug.ValidID(p.ID) {
		if want := p.ID[:10]; created.Format(time.DateOnly) != want {
			c.add(p.Path, "E-CREATED", "created %s differs from the date in the id, %s", created.Format(time.DateOnly), want)
		}
	}
	if v := f.String("parent"); v != "" {
		c.resolveRef(p, "parent", v)
	}
	for _, v := range f.Strings("sources") {
		c.resolveRef(p, "sources", v)
	}
	if v := f.String("superseded_by"); v != "" {
		c.resolveRef(p, "superseded_by", v)
	}

	refs, fileRefs := links.Extract(p.body)
	for _, r := range refs {
		line := p.bodyLine + r.Line - 1
		if ts := c.byID[r.Target]; len(ts) > 0 {
			for _, t := range ts {
				c.targets[p.Path] = append(c.targets[p.Path], t.Path)
			}
			continue
		}
		owners := c.byAlias[r.Target]
		switch {
		case len(owners) == 1:
			c.targets[p.Path] = append(c.targets[p.Path], owners[0].Path)
			if c.locked[p.Path] {
				c.addImmutable(p.Path, "W-ALIAS-LINK", "[[%s]] on line %d resolves through an alias to [[%s]]; a journal body is immutable, so this is not counted as a warning", r.Target, line, owners[0].ID)
			} else {
				c.add(p.Path, "W-ALIAS-LINK", "[[%s]] on line %d resolves through an alias; link [[%s]] instead", r.Target, line, owners[0].ID)
			}
		case len(owners) > 1:
			c.add(p.Path, "E-LINK", "[[%s]] on line %d is an alias of several entries (%s); link an id", r.Target, line, paths(owners, nil))
		case slug.ValidID(r.Target):
			c.add(p.Path, "E-LINK", "[[%s]] on line %d is not the id of an entry", r.Target, line)
		case links.Shaped(r.Target):
			c.brokenLink(p, r.Target, line)
		}
	}
	for _, r := range fileRefs {
		line := p.bodyLine + r.Line - 1
		dest := r.Path
		if i := strings.IndexAny(dest, "#?"); i >= 0 {
			dest = dest[:i]
		}
		if decoded, err := url.PathUnescape(dest); err == nil {
			dest = decoded
		}
		clean := path.Clean(dest)
		if clean != dest && clean+"/" != dest {
			c.add(p.Path, "E-FILE-LINK", "%s on line %d is not a plain path under files/; write it without . or ..", r.Path, line)
			continue
		}
		switch c.tree.Locate(clean) {
		case store.Outside:
			c.add(p.Path, "E-FILE-LINK", "%s on line %d is outside the store", r.Path, line)
		case store.Missing:
			c.add(p.Path, "E-FILE-LINK", "%s on line %d does not exist", r.Path, line)
		}
	}

	if p.Type.Name == "journal" {
		if sum := f.String("body_sha256"); sum != "" && sum != entry.BodySHA256(p.body) {
			c.add(p.Path, "E-HASH", "body does not match body_sha256; journal bodies are immutable, so restore the body, "+
				"or capture the corrected text and supersede with: memex set %s status=superseded superseded_by=<new>", p.ID)
		}
	}

	// Warnings.
	if len(f.Strings("tags")) == 0 && len(f.Strings("keywords")) == 0 && !isInvalid(f, "tags") && !isInvalid(f, "keywords") {
		c.add(p.Path, "W-NO-TERMS", "no tags or keywords; add the words a reader would search with: memex set %s keywords=<word>,<word>", p.ID)
	}
	if c.requireRepos && !f.Has("repos") {
		c.add(p.Path, "W-NO-REPO", "no repos, and this store requires them (require_repos in memex.toml); add the repository it concerns: "+
			"memex set %s repos=<name>, or if it concerns none: memex set %s repos=[]", p.ID, p.ID)
	}
	if !f.Has("summary") {
		c.add(p.Path, "W-NO-SUMMARY", "no summary; add one sentence that tells a reader whether to open the entry: memex set %s summary=<sentence>", p.ID)
	}
	if n := entry.ContentMeasure(p.body); n < entry.MinContent {
		if c.locked[p.Path] {
			c.addImmutable(p.Path, "W-SHORT-BODY", "body content measure %d is below %d; a journal body is immutable, so this is not counted as a warning; if knowledge is missing, capture a fuller entry and supersede with: memex set %s status=superseded superseded_by=<new>", n, entry.MinContent, p.ID)
		} else {
			c.add(p.Path, "W-SHORT-BODY", "body content measure %d is below %d; edit the file, then memex save", n, entry.MinContent)
		}
	}
	if p.Type.Name == "reference" {
		verified, ok := f.Date("verified")
		days, hasDays := f["review_days"].(int64)
		if ok && hasDays && verified.AddDate(0, 0, int(days)).Before(c.today) {
			c.add(p.Path, "W-STALE-REFERENCE", "verified %s is more than review_days (%d) ago; check the facts, then: memex set %s verified=%s",
				verified.Format(time.DateOnly), days, p.ID, c.today.Format(time.DateOnly))
		}
	}
	if due, ok := f.Date("due"); ok && spec.IsOpen(p.Type.Name, f.String("status")) && due.Before(c.today) {
		c.add(p.Path, "W-PAST-DUE", "%s", pastDueMessage(p.Type.Name, p.ID, due))
	}
	if p.Type.Name == "journal" {
		status := f.String("status")
		created, ok := f.Date("created")
		if status == "raw" && ok && created.AddDate(0, 0, rawOldDays).Before(c.today) {
			c.add(p.Path, "W-RAW-OLD", "raw since %s, more than %d days ago; review it, then: memex set %s status=kept, or memex promote %s",
				created.Format(time.DateOnly), rawOldDays, p.ID, p.ID)
		}
		if status == "promoted" && !cited[p.ID] {
			c.add(p.Path, "W-PROMOTED-UNCITED", "promoted, but no entry lists %s in sources", q(p.ID))
		}
	}
	if f["allow_secret"] != true {
		for _, s := range secrets(p.body) {
			c.add(p.Path, "W-SECRET", "line %d looks like a credential (%s); remove it; if it is not one, add the line %s, "+
				"or for a journal entry: memex set %s allow_secret=true", p.bodyLine+s.line-1, s.what, allowSecret, p.ID)
		}
	}
}

// q quotes a value in a message as a TOML basic string (SPEC §7).
func q(s string) string { return entry.Quote(s) }

func isInvalid(f entry.Fields, name string) bool {
	_, bad := f[name].(entry.Invalid)
	return bad
}

// checkCanonical adds W-CANONICAL for every entry with no errors that is not in canonical
// form.
func (c *checker) checkCanonical() {
	errs := map[string]bool{}
	for _, f := range c.findings {
		if IsError(f) {
			errs[f.Path] = true
		}
	}
	for _, p := range c.files {
		if p.fields == nil || errs[p.Path] {
			continue
		}
		if entry.Render(p.fields, entry.CanonicalBody(p.Type, p.body)) != string(p.data) {
			c.add(p.Path, "W-CANONICAL", "not in canonical form; run memex fmt %s", p.Path)
		}
	}
}

// selectStaged keeps the findings about staged paths (or inside staged directories' paths)
// and about the entries that staged entries link to.
func (c *checker) selectStaged(staged []string) []Finding {
	keep := map[string]bool{}
	for _, s := range staged {
		keep[s] = true
		for _, t := range c.targets[s] {
			keep[t] = true
		}
	}
	var out []Finding
	for _, f := range c.findings {
		if keep[f.Path] || slices.ContainsFunc(staged, func(s string) bool { return strings.HasPrefix(s, f.Path+"/") }) {
			out = append(out, f)
		}
	}
	return out
}

// pastDueMessage is the W-PAST-DUE message (SPEC §7): the commands that close the entry, move
// the date with its reason, and, for a task, park it.
func pastDueMessage(typ, id string, due time.Time) string {
	msg := fmt.Sprintf("due %s has passed; if it is done: memex update %s --status done --summary=<sentence> --body -; "+
		"to move the date, saying why in the body: memex update %s --due=<new> --summary=<sentence> --body -",
		due.Format(time.DateOnly), id, id)
	if typ == spec.ParkedType {
		msg += fmt.Sprintf("; to park it: memex update %s --status %s --summary=<sentence> --body -", id, spec.ParkedStatus)
	}
	return msg
}

// checkStale reports W-OPEN-STALE (SPEC §7): an open or in-progress task with neither due nor
// due_when, last changed more than stale_days days before today. The last change is today
// when the file read differs from HEAD's, and otherwise the committer date of the newest
// commit that changes it; a task HEAD does not hold has none. In a shallow clone, a task whose
// newest version is the boundary's was last changed on the boundary's date or earlier, so it
// is reported once that date is old enough, and the message says "or earlier".
func (c *checker) checkStale(p *parsed, hist *store.History) {
	status := p.fields.String("status")
	if c.staleDays == 0 || p.Type.Name != "task" || !spec.IsOpen(p.Type.Name, status) {
		return
	}
	for _, name := range spec.DueFields {
		if p.fields.Has(name) {
			return // a dated or waiting task says when it is owed; W-PAST-DUE covers a past date
		}
	}
	vs := hist.Versions[p.Path]
	if len(vs) == 0 || string(vs[0]) != string(p.data) {
		return // not in HEAD, or changed since: changed today
	}
	changed, err := time.Parse(time.DateOnly, hist.Changed[p.Path])
	if err != nil {
		return
	}
	if changed.AddDate(0, 0, int(c.staleDays)).Before(c.today) {
		since := changed.Format(time.DateOnly)
		if hist.Boundary[p.Path] {
			since += " or earlier (the history is shallow)"
		}
		c.add(p.Path, "W-OPEN-STALE", "%s and unchanged since %s, more than stale_days (%d) days ago; "+
			"record where it stands: memex update %s --summary=<sentence> --body -, "+
			"or close it: memex update %s --status done --summary=<sentence> --body -, "+
			"or park it: memex update %s --status %s --summary=<sentence> --body -, "+
			"or say when it is owed: memex update %s --due=<date> --summary=<sentence> --body -",
			status, since, c.staleDays, p.ID, p.ID, p.ID, spec.ParkedStatus, p.ID)
	}
}
