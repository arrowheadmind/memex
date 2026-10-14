package importv3

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/links"
	"github.com/arrowheadmind/memex/internal/slug"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// Refusal is a delta import refused before anything was written (SPEC §13.7), with its exit
// code.
type Refusal struct {
	Code int
	Msg  string
}

func (r *Refusal) Error() string { return r.Msg }

// DeltaReport is the `delta` member of a delta import's report (SPEC §13.7).
type DeltaReport struct {
	Added              []DeltaAdded
	Unchanged          int
	Changed            []idPair // present in the store with a different body; left alone
	Moved              []idPair // present in the store under another id
	AttachmentsAdded   []string
	AttachmentsChanged []string
	AliasesDropped     []idPair // ID is the new entry, Fresh the alias
}

// DeltaAdded is one entry a delta import wrote.
type DeltaAdded struct{ ID, Fresh, Type, Path string }

// idPair is a store id and the id the fresh import gave the same unit.
type idPair struct{ ID, Fresh string }

func pairs(ps []idPair, a, b string) jsonx.Array {
	out := jsonx.Array{}
	for _, p := range ps {
		out = append(out, jsonx.Object{{Key: a, Value: jsonx.String(p.ID)}, {Key: b, Value: jsonx.String(p.Fresh)}})
	}
	return out
}

func (d *DeltaReport) json() jsonx.Value {
	added := jsonx.Array{}
	for _, a := range d.Added {
		added = append(added, jsonx.Object{
			{Key: "id", Value: jsonx.String(a.ID)}, {Key: "fresh_id", Value: jsonx.String(a.Fresh)},
			{Key: "type", Value: jsonx.String(a.Type)}, {Key: "path", Value: jsonx.String(a.Path)},
		})
	}
	return jsonx.Object{
		{Key: "added", Value: added},
		{Key: "unchanged", Value: jsonx.Int(d.Unchanged)},
		{Key: "changed", Value: pairs(d.Changed, "id", "fresh_id")},
		{Key: "moved", Value: pairs(d.Moved, "id", "fresh_id")},
		{Key: "attachments_added", Value: jsonx.Strings(d.AttachmentsAdded)},
		{Key: "attachments_changed", Value: jsonx.Strings(d.AttachmentsChanged)},
		{Key: "aliases_dropped", Value: pairs(d.AliasesDropped, "id", "alias")},
	}
}

// ReportName returns the first free report name in dir for today: import-report-<today>.json,
// then -2, -3, … (SPEC §13.7).
func ReportName(dir, today string) string {
	name := "import-report-" + today + ".json"
	for n := 2; ; n++ {
		if _, err := os.Lstat(filepath.Join(dir, name)); errors.Is(err, fs.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("import-report-%s-%d.json", today, n)
	}
}

// Delta runs the import into an existing store (SPEC §13.7): a fresh import of the sources
// into a temporary directory, then only the entries the store does not hold yet are added,
// with their links mapped to the store's ids. Nothing present is changed. It returns the
// report, the report's file name in the store, and a *Failure when there are problems (the
// report is written either way) or a *Refusal when nothing was written.
func Delta(o Options, into string) (*Report, *DeltaReport, string, error) {
	g := gitx.New(into, 0)
	if top, err := g.Run("rev-parse", "--show-toplevel"); err != nil {
		return nil, nil, "", &Refusal{spec.ExitFailure, fmt.Sprintf("%s is not a git repository", into)}
	} else if real, err := filepath.EvalSymlinks(into); err != nil || filepath.Clean(strings.TrimSpace(top)) != filepath.Clean(real) {
		return nil, nil, "", &Refusal{spec.ExitFailure, fmt.Sprintf("%s is not the top of its git repository", into)}
	}
	if _, _, err := store.ReadManifest(store.WorkTree{Root: into}); err != nil {
		var fe *store.FormatError
		if errors.As(err, &fe) {
			return nil, nil, "", &Refusal{spec.ExitUsage, fe.Msg}
		}
		return nil, nil, "", &Refusal{spec.ExitFailure, fmt.Sprintf("%s is not a memex store: %v", into, err)}
	}
	clean := func() error {
		out, err := g.Run("status", "--porcelain", "--untracked-files=all")
		if err != nil {
			return &Refusal{spec.ExitFailure, err.Error()}
		}
		if strings.TrimSpace(out) != "" {
			return &Refusal{spec.ExitFailure, fmt.Sprintf("%s has uncommitted changes; commit them first, so that the import is the only change to review", into)}
		}
		return nil
	}
	if err := clean(); err != nil {
		return nil, nil, "", err
	}

	tmp, err := os.MkdirTemp("", "memex-import-")
	if err != nil {
		return nil, nil, "", err
	}
	defer os.RemoveAll(tmp)
	fo := o
	fo.Out = filepath.Join(tmp, "fresh")
	report, err := Run(fo)
	var failure *Failure
	if err != nil && !errors.As(err, &failure) {
		return nil, nil, "", err
	}

	lockF, err := write.Lock(into, write.LockWait)
	if err != nil {
		var le *write.LockError
		if errors.As(err, &le) {
			return nil, nil, "", &Refusal{spec.ExitLock, le.Error()}
		}
		return nil, nil, "", err
	}
	defer lockF.Close()
	if err := clean(); err != nil {
		return nil, nil, "", err
	}
	name := ReportName(into, report.Today)
	d := &DeltaReport{}
	if failure != nil {
		// The fresh import does not balance: nothing is merged, the report says why.
		report.Delta = d
		if err := os.WriteFile(filepath.Join(into, name), report.JSON(), 0o644); err != nil {
			return nil, nil, "", err
		}
		return report, d, name, failure
	}
	if err := merge(report, d, fo.Out, into, o.Today); err != nil {
		return nil, nil, "", err
	}
	report.Delta = d
	if err := os.WriteFile(filepath.Join(into, name), report.JSON(), 0o644); err != nil {
		return nil, nil, "", err
	}
	if len(report.Problems) > 0 {
		return report, d, name, &Failure{Problems: report.Problems}
	}
	return report, d, name, nil
}

// merge adds to the store at into the entries and attachments of the fresh import at fresh
// that it does not hold yet (SPEC §13.7), and records what it did in d. Validation errors
// of the result are appended to the report's problems.
func merge(report *Report, d *DeltaReport, fresh, into string, today time.Time) error {
	freshEntries, _, err := store.LoadEntries(store.WorkTree{Root: fresh})
	if err != nil {
		return err
	}
	have, skipped, err := store.LoadEntries(store.WorkTree{Root: into})
	if err != nil {
		return err
	}
	byFresh := map[string]*store.Entry{}
	for _, e := range freshEntries {
		byFresh[e.ID] = e
	}
	// The fresh entries in unit order (the report's order), which decides who claims what.
	var order []*store.Entry
	for _, r := range report.Entries {
		if e := byFresh[r.ID]; e != nil {
			order = append(order, e)
			delete(byFresh, r.ID)
		}
	}
	var rest []string
	for id := range byFresh {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	for _, id := range rest {
		order = append(order, byFresh[id])
	}

	storeID := map[string]*store.Entry{}
	holders := map[string][]*store.Entry{} // alias → entries holding it
	taken := map[string]bool{}             // every id and alias present in the store
	for _, e := range have {
		storeID[e.ID] = e
		taken[e.ID] = true
		for _, a := range e.Fields.Strings("aliases") {
			holders[a] = append(holders[a], e)
			taken[a] = true
		}
	}
	for _, s := range skipped { // an unreadable entry still holds its id
		id := strings.TrimSuffix(filepath.Base(s.Path), ".md")
		taken[id] = true
	}

	// 1. Identity is the alias set: a fresh entry is present when a store entry holds one of
	// its aliases. A store entry is claimed once, preferring the one with the same id.
	claimed := map[*store.Entry]bool{}
	match := map[*store.Entry]*store.Entry{}
	for _, f := range order {
		var cands []*store.Entry
		for _, a := range f.Fields.Strings("aliases") {
			for _, h := range holders[a] {
				if !claimed[h] {
					cands = append(cands, h)
				}
			}
		}
		if len(cands) == 0 {
			continue
		}
		pick := cands[0]
		for _, c := range cands {
			if c.ID == f.ID {
				pick = c
				break
			}
		}
		claimed[pick] = true
		match[f] = pick
	}
	// 2. Without an alias in common, the same id with the same type and body is present too
	// (an entry whose aliases were edited away).
	for _, f := range order {
		if match[f] != nil {
			continue
		}
		if s := storeID[f.ID]; s != nil && !claimed[s] && s.Type == f.Type && s.Body == f.Body {
			claimed[s] = true
			match[f] = s
		}
	}
	// 3. Every other fresh entry is new. Its id is kept unless the store has it, in which case
	// it gets the first free -2, -3, … suffix.
	idMap := map[string]string{}
	for _, f := range order {
		if s := match[f]; s != nil {
			idMap[f.ID] = s.ID
		}
	}
	used := map[string]bool{}
	for id := range taken {
		used[id] = true
	}
	var added []*store.Entry
	for _, f := range order {
		if match[f] != nil {
			continue
		}
		id := f.ID
		if used[id] {
			id = freeID(f.ID, used)
		}
		used[id] = true
		idMap[f.ID] = id
		added = append(added, f)
	}

	// Matched entries: compared, never written.
	for _, f := range order {
		s := match[f]
		if s == nil {
			continue
		}
		if s.ID != f.ID {
			d.Moved = append(d.Moved, idPair{s.ID, f.ID})
		}
		if mapBody(f.Body, idMap) == s.Body {
			d.Unchanged++
		} else {
			d.Changed = append(d.Changed, idPair{s.ID, f.ID})
		}
	}

	// New entries: links and references mapped, aliases that the store holds dropped.
	for _, f := range added {
		id := idMap[f.ID]
		fields := entry.Fields{}
		for k, v := range f.Fields {
			fields[k] = v
		}
		for _, k := range []string{"superseded_by", "parent"} {
			if v, ok := fields[k].(string); ok && idMap[v] != "" {
				fields[k] = idMap[v]
			}
		}
		if src := f.Fields.Strings("sources"); len(src) > 0 {
			var out []string
			for _, s := range src {
				if idMap[s] != "" {
					s = idMap[s]
				}
				out = append(out, s)
			}
			fields["sources"] = out
		}
		var aliases []string
		for _, a := range f.Fields.Strings("aliases") {
			if used[a] {
				d.AliasesDropped = append(d.AliasesDropped, idPair{id, a})
				continue
			}
			used[a] = true
			aliases = append(aliases, a)
		}
		if len(aliases) > 0 {
			fields["aliases"] = aliases
		} else {
			delete(fields, "aliases")
		}
		body := mapBody(f.Body, idMap)
		if f.Type.Name == "journal" {
			fields["body_sha256"] = entry.BodySHA256(body)
		}
		rel := f.Type.Directory + "/" + id + ".md"
		full := filepath.Join(into, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(entry.Render(fields, body)), 0o644); err != nil {
			return err
		}
		d.Added = append(d.Added, DeltaAdded{ID: id, Fresh: f.ID, Type: f.Type.Name, Path: rel})
	}

	// Attachments: a file under files/<owner>/ is copied when the store has no file at that
	// path (the owner's id mapped); one that differs is left alone.
	filesDir := filepath.Join(fresh, "files")
	err = filepath.WalkDir(filesDir, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if de.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(filesDir, p)
		rel = filepath.ToSlash(rel)
		owner, below, _ := strings.Cut(rel, "/")
		if m := idMap[owner]; m != "" {
			owner = m
		}
		dest := "files/" + owner + "/" + below
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		full := filepath.Join(into, filepath.FromSlash(dest))
		if old, err := os.ReadFile(full); err == nil {
			if !bytes.Equal(old, data) {
				d.AttachmentsChanged = append(d.AttachmentsChanged, dest)
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
		d.AttachmentsAdded = append(d.AttachmentsAdded, dest)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(d.AttachmentsAdded)
	sort.Strings(d.AttachmentsChanged)

	// The result must validate like any import (SPEC §13.1): errors are problems.
	findings, err := check.Run(store.WorkTree{Root: into}, today, nil)
	if err != nil {
		return err
	}
	for _, f := range findings {
		if check.IsError(f) {
			report.Problems = append(report.Problems, fmt.Sprintf("check: %s: %s %s", f.Path, f.Code, f.Message))
		}
	}
	return nil
}

// freeID returns base with the first suffix -2, -3, … that is not used, within the id
// length cap (SPEC §5.4).
func freeID(base string, used map[string]bool) string {
	for n := 2; ; n++ {
		s := fmt.Sprintf("-%d", n)
		id := base
		if len(id)+len(s) > slug.MaxIDLength {
			id = strings.TrimRight(id[:slug.MaxIDLength-len(s)], "-")
		}
		if !used[id+s] {
			return id + s
		}
	}
}

var filesPrefix = regexp.MustCompile(`files/[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9-]+/`)

var wikiTarget = regexp.MustCompile(`\[\[([^\]|\n]+)((?:\|[^\]\n]*)?)\]\]`)

// mapBody rewrites the [[TARGET]] links of body whose target is a fresh id mapped to another
// id, outside fenced code and code spans (SPEC §6), and the files/<id>/ prefixes of mapped
// owners. Everything else is kept byte for byte.
func mapBody(body string, idMap map[string]string) string {
	lines := entry.Lines(body)
	fenced := entry.Fenced(lines)
	changed := false
	for i, line := range lines {
		if fenced[i] {
			continue
		}
		visible := links.StripCodeSpans(line)
		ms := wikiTarget.FindAllStringSubmatchIndex(visible, -1)
		if len(ms) == 0 && !strings.Contains(line, "files/") {
			continue
		}
		var b strings.Builder
		last := 0
		for _, m := range ms {
			target := line[m[2]:m[3]]
			to := idMap[target]
			if to == "" || to == target {
				continue
			}
			b.WriteString(line[last:m[2]])
			b.WriteString(to)
			last = m[3]
		}
		b.WriteString(line[last:])
		out := filesPrefix.ReplaceAllStringFunc(b.String(), func(m string) string {
			if to := idMap[m[len("files/"):len(m)-1]]; to != "" {
				return "files/" + to + "/"
			}
			return m
		})
		if out != line {
			lines[i] = out
			changed = true
		}
	}
	if !changed {
		return body
	}
	out := strings.Join(lines, "\n")
	if strings.HasSuffix(body, "\n") {
		out += "\n"
	}
	return out
}
