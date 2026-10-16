package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/importv3"
	"github.com/arrowheadmind/memex/internal/slug"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// NewOptions are the options of `memex new`, `capture` and the new entry of `promote`
// (SPEC §11.2, §11.8). Empty strings and nil slices are absent options.
type NewOptions struct {
	Type                                         *spec.Type
	Title, Summary, Body, From                   string
	TitleSet, SummarySet                         bool
	Status, Kind, Parent, Verified, ReviewDays   string
	Tags, Keywords, Repos, Hosts, Sources, Alias []string
	NoRepo, Always                               bool
}

// typeOptions are the new options that set a field allowed only on some types.
var typeOptions = []struct{ option, field string }{
	{"--always", "always"}, {"--kind", "kind"}, {"--parent", "parent"},
	{"--verified", "verified"}, {"--review-days", "review_days"},
}

func (o *NewOptions) given(option string) bool {
	switch option {
	case "--always":
		return o.Always
	case "--kind":
		return o.Kind != ""
	case "--parent":
		return o.Parent != ""
	case "--verified":
		return o.Verified != ""
	case "--review-days":
		return o.ReviewDays != ""
	}
	return false
}

// unknownStatus describes a status that the type does not have (SPEC §17.4).
func unknownStatus(typ *spec.Type, s string) string {
	problem := fmt.Sprintf("unknown status %q for a %s", s, typ.Name)
	if near, ok := spec.Nearest(s, typ.Statuses); ok {
		return problem + fmt.Sprintf("; did you mean %q?", near)
	}
	return problem + "; valid: " + strings.Join(typ.Statuses, ", ")
}

// checkUsage validates the options that need no store (SPEC §11.2 step 1).
func (o *NewOptions) checkUsage() *UsageError {
	if o.Status != "" && !slices.Contains(o.Type.Statuses, o.Status) {
		return &UsageError{unknownStatus(o.Type, o.Status)}
	}
	for _, t := range typeOptions {
		f := spec.LookupField(t.field)
		if o.given(t.option) && !f.AllowedOnType(o.Type.Name) {
			return &UsageError{fmt.Sprintf("option %s is not allowed on a %s; it is allowed on: %s",
				t.option, o.Type.Name, strings.Join(f.AllowedOn, ", "))}
		}
	}
	if o.Verified != "" {
		if _, ok := config.ParseDate(o.Verified); !ok {
			return &UsageError{fmt.Sprintf("value for --verified must be a date written YYYY-MM-DD, not %q", o.Verified)}
		}
	}
	if o.ReviewDays != "" {
		if _, err := strconv.ParseInt(o.ReviewDays, 10, 64); err != nil || strings.HasPrefix(o.ReviewDays, "+") {
			return &UsageError{fmt.Sprintf("value for --review-days must be an integer, not %q", o.ReviewDays)}
		}
	}
	return nil
}

// readBody reads --body FILE, or standard input for "-".
func (e *Env) readBody(name string) (string, error) {
	if name == "-" {
		b, err := io.ReadAll(e.Stdin)
		return string(b), err
	}
	b, err := os.ReadFile(name)
	return string(b), err
}

// fromFile builds the title, summary and body of `capture --from PATH` (SPEC §11.2).
func (o *NewOptions) fromFile(content string, today time.Time) (title, summary, body string) {
	title, summary = "", "Imported from "+o.From
	lines := entry.Lines(strings.ReplaceAll(content, "\r\n", "\n"))
	fenced := entry.Fenced(lines)
	for i, l := range lines {
		if fenced[i] {
			continue
		}
		if t, ok := headingText(l); ok && t != "" {
			title = t
			break
		}
	}
	// With no ATX heading, a leading YAML block gives the title and summary (SPEC §13.3).
	if title == "" {
		if name, desc := importv3.YAMLBlock(lines); name != "" {
			title = name
			if desc != "" {
				summary = desc
			}
		}
	}
	if title == "" {
		title = filepath.Base(o.From)
	}
	body = fmt.Sprintf("Imported from `%s` (%s).\n\n%s", o.From, today.Format(time.DateOnly), content)
	return title, summary, body
}

// headingText returns the text of an ATX heading line, without its closing sequence.
func headingText(line string) (string, bool) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return "", false
	}
	n := len(t) - len(strings.TrimLeft(t, "#"))
	if n < 1 || n > 6 || (len(t) > n && t[n] != ' ' && t[n] != '\t') {
		return "", false
	}
	t = strings.TrimSpace(t[n:])
	if c := strings.TrimRight(t, "#"); c != t && (c == "" || strings.HasSuffix(c, " ") || strings.HasSuffix(c, "\t")) {
		t = strings.TrimSpace(c)
	}
	return t, true
}

// attribution is the default `repos` of a new entry (SPEC §5.5): the repository containing
// the current directory, named by the parent of its common git directory, unless that
// repository is the store. This is memex's only use of the current directory.
func attribution(root string) []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	out, err := gitx.New(cwd, 0).Run("rev-parse", "--git-common-dir")
	if err != nil {
		return nil
	}
	common := strings.TrimSuffix(out, "\n")
	if !filepath.IsAbs(common) {
		common = filepath.Join(cwd, common)
	}
	repo := filepath.Dir(filepath.Clean(common))
	realRepo, err1 := filepath.EvalSymlinks(repo)
	realRoot, err2 := filepath.EvalSymlinks(root)
	if err1 == nil && err2 == nil && realRepo == realRoot {
		return nil
	}
	name := filepath.Base(repo)
	if name == "/" || name == "." {
		return nil
	}
	return []string{name}
}

// prepared is a new entry computed in full before anything is written.
type prepared struct {
	typ    *spec.Type
	id     string
	path   string
	fields entry.Fields
	body   string
}

func (p *prepared) data() []byte { return []byte(entry.Render(p.fields, p.body)) }

// prepareBody reads and normalizes the body and checks its content measure (SPEC §11.2
// step 2). It returns the title and summary too, which --from may supply.
func (e *Env) prepareBody(o *NewOptions, today time.Time) (title, summary, body string, code int, uerr *UsageError) {
	title, summary = o.Title, o.Summary
	if o.From != "" {
		name, err := config.Expand(o.From, e.Getenv("HOME")) // ~/ and $HOME as in §3.1
		if err != nil {
			name = o.From
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return "", "", "", 0, &UsageError{fmt.Sprintf("cannot read --from %s: %v", o.From, unwrap(err))}
		}
		t, s, b := o.fromFile(string(content), today)
		if !o.TitleSet {
			title = t
		}
		if !o.SummarySet {
			summary = s
		}
		body = b
	} else {
		b, err := e.readBody(o.Body)
		if err != nil {
			return "", "", "", 0, &UsageError{fmt.Sprintf("cannot read --body %s: %v", o.Body, unwrap(err))}
		}
		body = b
	}
	body = entry.Normalize(body)
	if n := entry.ContentMeasure(body); n < entry.MinContent {
		return "", "", "", e.fail(spec.ExitFailure,
			"body content measure %d is below %d: a body needs at least %d non-whitespace characters outside headings, "+
				"the knowledge itself and not only its outline; see memex help entries", n, entry.MinContent, entry.MinContent), nil
	}
	return title, summary, body, 0, nil
}

func unwrap(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// build computes the new entry's fields (SPEC §5.3, §5.5, §11.2). Field references are
// resolved first; an unknown one is reported and build returns nil.
func (o *NewOptions) build(w *writing, res *resolver, title, summary, body string, repos []string) *prepared {
	f := entry.Fields{
		"title":   title,
		"status":  o.Status,
		"created": w.today,
		"summary": summary,
	}
	if o.Status == "" {
		f["status"] = o.Type.Statuses[0]
	}
	setArray := func(name string, v []string) {
		if len(v) > 0 {
			f[name] = v
		}
	}
	setArray("tags", o.Tags)
	setArray("keywords", o.Keywords)
	switch {
	case len(o.Repos) > 0:
		setArray("repos", o.Repos)
	case !o.NoRepo:
		setArray("repos", repos)
	}
	setArray("hosts", o.Hosts)
	if o.Always {
		f["always"] = true
	}
	if o.Kind != "" {
		f["kind"] = o.Kind
	}
	if o.Parent != "" {
		p := res.resolve(w.e, o.Parent)
		if p == nil {
			return nil
		}
		f["parent"] = p.ID
	}
	var sources []string
	for _, s := range o.Sources {
		p := res.resolve(w.e, s)
		if p == nil {
			return nil
		}
		if !slices.Contains(sources, p.ID) {
			sources = append(sources, p.ID)
		}
	}
	setArray("sources", sources)
	if o.Type.Name == "reference" {
		f["verified"] = w.today
		if d, ok := config.ParseDate(o.Verified); ok {
			f["verified"] = d
		}
	}
	if o.ReviewDays != "" {
		n, _ := strconv.ParseInt(o.ReviewDays, 10, 64)
		f["review_days"] = n
	}
	setArray("aliases", o.Alias)
	if o.Type.Name == "journal" {
		f["body_sha256"] = entry.BodySHA256(body)
	}
	id := slug.ID(w.today.Format(time.DateOnly), title)
	return &prepared{typ: o.Type, id: id, path: o.Type.Directory + "/" + id + ".md", fields: f, body: body}
}

// existing finds an entry file with id in any type directory, and its parsed entry when it
// can be read.
func existing(root string, entries []*store.Entry, id string) (*store.File, *store.Entry) {
	layout, err := store.Enumerate(store.WorkTree{Root: root})
	if err != nil {
		panic(err)
	}
	for i := range layout.Files {
		f := &layout.Files[i]
		if f.ID != id {
			continue
		}
		for _, en := range entries {
			if en.Path == f.Path {
				return f, en
			}
		}
		return f, nil
	}
	return nil, nil
}

// refuseExisting is the refusal of SPEC §11.2 step 3 for an id that exists with another body
// or type.
func (w *writing) refuseExisting(f *store.File, en *store.Entry) int {
	title, summary := "", ""
	if en != nil {
		title, summary = en.Fields.String("title"), en.Fields.String("summary")
	}
	w.end(nil)
	fmt.Fprintf(w.e.Stderr, "memex: exists %s\n  title: %s\n  summary: %s\n"+
		"To read it: memex show %s. To add to it: edit the file, then memex save.\n"+
		"To create a different entry: choose a more specific --title.\n", f.Path, title, summary, f.ID)
	return spec.ExitFailure
}

// New runs `memex new` and `memex capture` (SPEC §11.2).
func New(e *Env, o NewOptions) (int, *UsageError) {
	if uerr := o.checkUsage(); uerr != nil {
		return 0, uerr
	}
	today, err := config.Today(e.Getenv, e.Now())
	if err != nil {
		return e.failErr(err), nil
	}
	title, summary, body, code, uerr := e.prepareBody(&o, today)
	if uerr != nil || code != 0 {
		return code, uerr
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	var repos []string
	if len(o.Repos) == 0 && !o.NoRepo {
		repos = attribution(w.root)
	}
	home := e.Getenv("HOME")
	if o.From != "" {
		// The imported file's path resolves to the entry, in its alias form (SPEC §11.2).
		if a := config.PathAlias(o.From, home); !slices.Contains(o.Alias, a) {
			o.Alias = append(o.Alias, a)
		}
	}
	entries := w.entries()
	if o.From != "" {
		// A file already brought in, by capture --from or by the importer's --leak (SPEC
		// §13.6), is an alias of its entry; capturing it again is the §11.2 no-op.
		if en := fromAlias(entries, o.From, home); en != nil {
			return w.nothing("exists "+en.Path, "remove "+o.From+" after review"), nil
		}
	}
	p := o.build(w, newResolver(entries), title, summary, body, repos)
	if p == nil {
		w.end(nil)
		return spec.ExitFailure, nil
	}
	var removeLine []string
	if o.From != "" {
		removeLine = []string{"remove " + o.From + " after review"}
	}
	if f, en := existing(w.root, entries, p.id); f != nil {
		if f.Type == p.typ && en != nil && en.Body == p.body {
			return w.nothing(append([]string{"exists " + f.Path}, removeLine...)...), nil
		}
		return w.refuseExisting(f, en), nil
	}
	files := []write.File{{Path: p.path, Data: p.data(), Check: true}}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return w.e.failedValidation(err), nil
	}
	msg := fmt.Sprintf("memex: new %s %s", p.typ.Name, p.id)
	return w.commit(files, msg, append([]string{"created " + p.path}, removeLine...)...), nil
}

// failedValidation reports a step 3 refusal.
func (e *Env) failedValidation(err error) int {
	if ve, ok := err.(*write.ValidationError); ok {
		return e.validationFailed(ve)
	}
	panic(err)
}

// fromAlias returns the entry that holds the --from path as an alias, as given or in the
// same alias form (SPEC §11.2).
func fromAlias(entries []*store.Entry, from, home string) *store.Entry {
	for _, en := range entries {
		for _, a := range en.Fields.Strings("aliases") {
			if config.AliasNamesPath(a, from, home) {
				return en
			}
		}
	}
	return nil
}
