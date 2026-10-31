package commands

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/write"
)

// UpdateOptions are the arguments of `memex update` (SPEC §11.17).
type UpdateOptions struct {
	ID, Summary, Body, Heading, Status string
	StatusSet                          bool // --status was given, possibly empty
	NoRepo                             bool
}

// DefaultHeading is the heading of an update section without --heading (SPEC §11.17).
const DefaultHeading = "Status"

// MaxHeading bounds --heading, in code points (SPEC §11.17).
const MaxHeading = 100

// CheckHeading reports why a --heading value is a usage error, or "".
func CheckHeading(h string) string {
	if n := utf8.RuneCountInString(h); n < 1 || n > MaxHeading || strings.ContainsAny(h, "\n\r") {
		return fmt.Sprintf("value for --heading must be 1 to %d characters on one line", MaxHeading)
	}
	return ""
}

// shortBody is the refusal of a body below the content measure (SPEC §5.6), or "".
func shortBody(body string) string {
	if n := entry.ContentMeasure(body); n < entry.MinContent {
		return fmt.Sprintf("body content measure %d is below %d: a body needs at least %d non-whitespace characters outside headings, "+
			"the knowledge itself and not only its outline; see memex help entries", n, entry.MinContent, entry.MinContent)
	}
	return ""
}

// Update runs `memex update` (SPEC §11.17): a dated section appended to the body and a new
// summary, with an optional status, in one commit. The checks run in the order of SPEC §11.7,
// refusal order: the id, the entry, then the options in synopsis order.
func Update(e *Env, o UpdateOptions) (int, *UsageError) {
	if o.Heading == "" {
		o.Heading = DefaultHeading
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	res := newResolver(w.entries())
	en, r := w.target(res, o.ID)
	if r != nil {
		return w.refuse(spec.ExitFailure, "%s", r.msg), nil
	}
	if en.Type.Name == "journal" {
		return w.refuse(spec.ExitFailure, "journal bodies are immutable; record the progress with memex capture, "+
			"then memex set %s status=superseded superseded_by=<new> if it replaces this one", en.ID), nil
	}
	// The entry: uncommitted changes, then fields that cannot be kept (planFields, with no
	// assignment yet).
	if _, _, r := planFields(w, res, en, nil); r != nil {
		return w.refuse(spec.ExitFailure, "%s", r.msg), nil
	}
	if o.Summary == en.Fields.String("summary") {
		return w.refuse(spec.ExitFailure, "--summary is the current summary of %s; write the summary as it stands after this section", en.ID), nil
	}
	text, err := e.readBody(o.Body)
	if err != nil {
		w.end(nil)
		return 0, &UsageError{fmt.Sprintf("cannot read --body %s: %v", o.Body, unwrap(err))}
	}
	section := entry.Normalize(text)
	if why := shortBody(section); why != "" {
		return w.refuse(spec.ExitFailure, "%s", why), nil
	}
	if o.StatusSet && !slices.Contains(en.Type.Statuses, o.Status) {
		w.end(nil)
		return 0, &UsageError{unknownStatus(en.Type, o.Status)}
	}
	// The repository requirement, after the options (SPEC §5.5, §11.17 step 8).
	noRepo := w.requireRepos() && !en.Fields.Has("repos")
	if noRepo && !o.NoRepo {
		return w.refuseNoRepo("%s", updateRefusalNoRepo(w.root, en.ID)), nil
	}
	asgs := []Assignment{{Field: "summary", Op: "=", Value: o.Summary, field: spec.LookupField("summary")}}
	if o.StatusSet {
		asgs = append(asgs, Assignment{Field: "status", Op: "=", Value: o.Status, field: spec.LookupField("status")})
	}
	f, _, r := planFields(w, res, en, asgs)
	if r != nil {
		w.end(nil)
		if r.usage {
			return 0, &UsageError{r.msg}
		}
		return e.fail(spec.ExitFailure, "%s", r.msg), nil
	}
	if noRepo {
		f["repos"] = []string{}
	}
	body := strings.TrimRightFunc(en.Body, unicode.IsSpace)
	if body != "" {
		body += "\n\n"
	}
	body += "## " + o.Heading + " " + w.today.Format("2006-01-02") + "\n\n" + section
	msg := "memex: update " + en.ID
	if s := f.String("status"); s != en.Status() {
		msg += " status=" + s
	}
	files := []write.File{{Path: en.Path, Data: []byte(entry.Render(f, body)), Check: true, Expect: en.Data}}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return e.failedValidation(err), nil
	}
	return w.commit(files, msg), nil
}
