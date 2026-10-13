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
	// targets records, for each entry path, the paths its links resolve to (for --staged).
	targets map[string][]string
}

func (c *checker) add(p, code, format string, args ...any) {
	c.findings = append(c.findings, Finding{Path: p, Code: code, Message: fmt.Sprintf(format, args...)})
}

// Run validates the store in tree. With staged non-nil, only findings about the staged paths
// and the entries their links resolve to are returned (SPEC §7, --staged). A manifest format
// this memex cannot read is a *store.FormatError.
func Run(tree store.Tree, today time.Time, staged []string) ([]Finding, error) {
	_, mf, err := store.ReadManifest(tree)
	if err != nil {
		return nil, err
	}
	layout, err := store.Enumerate(tree)
	if err != nil {
		return nil, err
	}
	c := &checker{tree: tree, today: today, byID: map[string][]*parsed{},
		byAlias: map[string][]*parsed{}, targets: map[string][]string{}}
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
		c.checkEntry(p, cited)
	}
	c.checkCanonical()
	if staged != nil {
		c.findings = c.selectStaged(staged)
	}
	Sort(c.findings)
	return c.findings, nil
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
			c.add(p.Path, "W-ALIAS-LINK", "[[%s]] on line %d resolves through an alias; link [[%s]] instead", r.Target, line, owners[0].ID)
		case len(owners) > 1:
			c.add(p.Path, "E-LINK", "[[%s]] on line %d is an alias of several entries (%s); link an id", r.Target, line, paths(owners, nil))
		case slug.ValidID(r.Target):
			c.add(p.Path, "E-LINK", "[[%s]] on line %d is not the id of an entry", r.Target, line)
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
	if !f.Has("summary") {
		c.add(p.Path, "W-NO-SUMMARY", "no summary; add one sentence that tells a reader whether to open the entry: memex set %s summary=<sentence>", p.ID)
	}
	if n := entry.ContentMeasure(p.body); n < entry.MinContent {
		c.add(p.Path, "W-SHORT-BODY", "body content measure %d is below %d; edit the file, then memex save", n, entry.MinContent)
	}
	if p.Type.Name == "reference" {
		verified, ok := f.Date("verified")
		days, hasDays := f["review_days"].(int64)
		if ok && hasDays && verified.AddDate(0, 0, int(days)).Before(c.today) {
			c.add(p.Path, "W-STALE-REFERENCE", "verified %s is more than review_days (%d) ago; check the facts, then: memex set %s verified=%s",
				verified.Format(time.DateOnly), days, p.ID, c.today.Format(time.DateOnly))
		}
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
	for _, s := range secrets(p.body) {
		c.add(p.Path, "W-SECRET", "line %d looks like a credential (%s); remove it, or add the line %s if it is not one",
			p.bodyLine+s.line-1, s.what, allowSecret)
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
