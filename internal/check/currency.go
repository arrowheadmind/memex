package check

import (
	"slices"
	"strings"
	"unicode"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
)

// CurrencyTypes are the types whose summaries W-STALE-SUMMARY checks (SPEC §7, summary
// currency).
var CurrencyTypes = []string{"task", "plan"}

// CurrencyDirs are the directories of CurrencyTypes, the paths whose history check reads.
func CurrencyDirs() []string {
	var dirs []string
	for _, t := range CurrencyTypes {
		dirs = append(dirs, spec.LookupType(t).Directory)
	}
	return dirs
}

// version is one version of an entry, as summary currency compares them.
type version struct {
	summary    string
	hasSummary bool
	status     string
	body       string
}

func readVersion(typ *spec.Type, id string, data []byte) (version, bool) {
	split, p := entry.SplitFile(string(data))
	if p != nil {
		return version{}, false
	}
	fields, _ := entry.ParseFields(split, typ, id)
	if fields == nil || !validSummary(fields) {
		return version{}, false
	}
	return version{
		summary: fields.String("summary"), hasSummary: fields.Has("summary"),
		status: fields.String("status"), body: split.Body,
	}, true
}

// validSummary reports whether fields has no summary or a valid one (SPEC §5.3): a summary of
// the wrong kind or outside its constraints makes a version unreadable for summary currency.
func validSummary(fields entry.Fields) bool {
	_, invalid := fields["summary"].(entry.Invalid)
	return !invalid
}

// headings returns the ATX heading lines of body that are outside fenced code blocks, in
// body order (SPEC §5.6).
func headings(body string) []string {
	lines := entry.Lines(body)
	fenced := entry.Fenced(lines)
	var out []string
	for i, l := range lines {
		if !fenced[i] && entry.IsATXHeading(l) {
			out = append(out, l)
		}
	}
	return out
}

// newSection returns the first heading line of newer, in body order, that it holds more times
// than older does.
func newSection(newer, older string) (string, bool) {
	count := map[string]int{}
	for _, h := range headings(older) {
		count[h]++
	}
	for _, h := range headings(newer) {
		if count[h] == 0 {
			return h, true
		}
		count[h]--
	}
	return "", false
}

// headingName is a heading line without its leading #s, the space after them and trailing
// whitespace.
func headingName(line string) string {
	t := strings.TrimLeft(line, " ")
	t = strings.TrimLeft(t, "#")
	t = strings.TrimPrefix(t, " ")
	return strings.TrimRightFunc(t, unicode.IsSpace)
}

// checkCurrency adds W-STALE-SUMMARY when the newest significant version of a task or plan
// is newer than its newest summary-changing one (SPEC §7, summary currency). committed holds
// the file in each commit that changes it, newest first; the current file is p.data.
func (c *checker) checkCurrency(p *parsed, committed [][]byte) {
	if !slices.Contains(CurrencyTypes, p.Type.Name) || !p.fields.Has("summary") || !validSummary(p.fields) {
		return
	}
	datas := committed
	if len(committed) == 0 || string(committed[0]) != string(p.data) {
		datas = append([][]byte{p.data}, committed...)
	}
	var vs []version
	for _, d := range datas {
		if v, ok := readVersion(p.Type, p.ID, d); ok {
			vs = append(vs, v)
		}
	}
	visible := func(s string) bool { return slices.Contains(p.Type.Visible, s) }
	for i := 0; i+1 < len(vs); i++ {
		newer, older := vs[i], vs[i+1]
		if newer.summary != older.summary || newer.hasSummary != older.hasSummary {
			return // the newest summary-changing version is at least as new as any significant one
		}
		if h, ok := newSection(newer.body, older.body); ok {
			c.add(p.Path, "W-STALE-SUMMARY", "summary was not updated when the section %s was added; "+
				"rewrite it to the current state: memex set %s summary=<sentence>", q(headingName(h)), p.ID)
			return
		}
		if visible(newer.status) != visible(older.status) {
			c.add(p.Path, "W-STALE-SUMMARY", "summary was not updated when status changed from %s to %s; "+
				"rewrite it to the current state: memex set %s summary=<sentence>", q(older.status), q(newer.status), p.ID)
			return
		}
	}
}
