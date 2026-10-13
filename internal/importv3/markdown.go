package importv3

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/arrowheadmind/memex/internal/entry"
)

// MaxTitle is the title length cap, in code points (SPEC §5.3, §13.3).
const MaxTitle = 300

var (
	atxHeading   = regexp.MustCompile(`^ {0,3}(#{1,6})(?: +(.*?))? *#* *$`)
	dateRe       = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	trailDateRe  = regexp.MustCompile(`\s*\((\d{4}-\d{2}-\d{2})\)\s*$`)
	foundDateRe  = regexp.MustCompile(`,\s*found (\d{4}-\d{2}-\d{2})\s*$`)
	tableSepRe   = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)
	mdLinkRe     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	filenameDate = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
	wordRe       = regexp.MustCompile(`[A-Za-z]+`)
)

// heading returns the level and text of an ATX heading line, or 0.
func heading(line string) (int, string) {
	m := atxHeading.FindStringSubmatch(line)
	if m == nil {
		return 0, ""
	}
	return len(m[1]), strings.TrimSpace(m[2])
}

// stripMarkup removes **, __, ~~, backticks and link syntax, keeping link text (SPEC §13.4).
func stripMarkup(s string) string {
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "").Replace(s)
	return s
}

// struck reports whether the text is fully struck through (~~…~~).
func struck(s string) bool {
	t := strings.TrimSpace(s)
	return len(t) > 4 && strings.HasPrefix(t, "~~") && strings.HasSuffix(t, "~~")
}

// cutTitle cuts a title to MaxTitle code points, then collapses whitespace.
func cutTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > MaxTitle {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:MaxTitle]))
	}
	if s == "" {
		return "untitled"
	}
	return s
}

// headingDate applies §13.3's date rule to a heading: a trailing "(YYYY-MM-DD)" or
// ", found YYYY-MM-DD" is removed and returned; else the last date in the text is returned
// and the text is kept. removed holds the text taken out, for the accounting.
func headingDate(text string) (title, date, removed string) {
	if m := trailDateRe.FindStringSubmatchIndex(text); m != nil {
		return strings.TrimSpace(text[:m[0]]), text[m[2]:m[3]], text[m[0]:]
	}
	if m := foundDateRe.FindStringSubmatchIndex(text); m != nil {
		return strings.TrimSpace(text[:m[0]]), text[m[2]:m[3]], text[m[0]:]
	}
	all := dateRe.FindAllString(text, -1)
	if len(all) > 0 {
		return text, all[len(all)-1], ""
	}
	return text, "", ""
}

// nonWS returns the non-whitespace code points of s, as a string, for the §13.6 comparison.
func nonWS(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.Is(unicode.White_Space, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// compareText is the §13.6 normalization of text before comparison: markup stripped,
// `\|` and `|` removed, whitespace removed.
func compareText(s string) string {
	s = stripMarkup(s)
	s = strings.ReplaceAll(s, `\|`, "")
	s = strings.ReplaceAll(s, "|", "")
	return nonWS(s)
}

// splitCells splits a table row on "|" not preceded by "\" and returns the cells trimmed,
// without the leading and trailing empty cells of a "| a | b |" row.
func splitCells(row string) []string {
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(row); i++ {
		c := row[i]
		if c == '\\' && i+1 < len(row) && row[i+1] == '|' {
			cur.WriteString(`\|`)
			i++
			continue
		}
		if c == '|' {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	cells = append(cells, cur.String())
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// unescapeCell turns `\|` into `|`.
func unescapeCell(s string) string { return strings.ReplaceAll(s, `\|`, "|") }

// isTableRow reports whether a line is a table row.
func isTableRow(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "|")
}

// firstWord returns the status value of a status line (SPEC §13.3): first word, lowercased,
// trailing punctuation removed.
func statusValue(line string) string {
	f := strings.Fields(line)
	if len(f) == 0 {
		return ""
	}
	w := strings.ToLower(f[0])
	w = strings.TrimRightFunc(w, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })
	return w
}

// tagify normalizes a tag (SPEC §13.3): lowercase, runs outside [a-z0-9] become "-",
// leading and trailing "-" removed.
func tagify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	return b.String()
}

// fenced marks the lines inside fenced code blocks (SPEC §5.6).
func fenced(lines []string) []bool { return entry.Fenced(lines) }

// prefixTags maps the heading prefixes of §13.3 to tags.
var prefixTags = []struct{ prefix, tag string }{
	{"Mistake / technique:", "technique"},
	{"Historical mistake", "historical"},
	{"Mistake:", "mistake"},
	{"Known Noise:", "noise"},
	{"Known Problem:", "problem"},
	{"Technique:", "technique"},
	{"Pattern:", "pattern"},
}

// stripPrefix removes a §13.3 heading prefix; it returns the rest, the tag and the removed
// text. "Historical mistake (superseded …):" keeps its parenthesis content in the title.
func stripPrefix(text string) (rest, tag, removed string) {
	for _, p := range prefixTags {
		if strings.HasPrefix(text, p.prefix) {
			if p.prefix == "Historical mistake" {
				// "Historical mistake (…): x" -> "(…): x"; only the words go
				return strings.TrimSpace(strings.TrimPrefix(text, p.prefix)), p.tag, p.prefix
			}
			return strings.TrimSpace(strings.TrimPrefix(text, p.prefix)), p.tag, p.prefix
		}
	}
	return text, "", ""
}

// titlePrefixes are the "# Word:" prefixes of file units (SPEC §13.3).
var titlePrefixes = []string{"Task:", "Investigation:", "Plan:", "Decision:", "Runbook:"}

// stripTitlePrefix removes "Task:" and friends from a first heading.
func stripTitlePrefix(text string) (rest, removed string) {
	for _, p := range titlePrefixes {
		if strings.HasPrefix(text, p) {
			return strings.TrimSpace(strings.TrimPrefix(text, p)), p
		}
	}
	return text, ""
}

// markers are the §13.4 marker words (case-insensitive, whole word) and their tags.
var markers = []struct{ word, tag string }{
	{"stale", "marker-stale"},
	{"resolved", "marker-resolved"},
	{"fixed", "marker-fixed"},
	{"no longer", "marker-no-longer"},
	{"retired", "marker-retired"},
	{"superseded", "marker-superseded"},
	{"moot", "marker-moot"},
	{"closed", "marker-closed"},
}

var markerRes = func() []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, m := range markers {
		w := strings.ReplaceAll(regexp.QuoteMeta(m.word), " ", `\s+`)
		out = append(out, regexp.MustCompile(`(?i)\b`+w+`\b`))
	}
	return out
}()

// markerTags returns the marker-* tags present in the text.
func markerTags(text string) []string {
	var out []string
	for i, re := range markerRes {
		if re.MatchString(text) {
			out = append(out, markers[i].tag)
		}
	}
	return out
}

// firstParagraph returns the first paragraph of a body (lines up to the first blank line).
func firstParagraph(body string) string {
	var out []string
	for _, l := range strings.Split(body, "\n") {
		if strings.TrimSpace(l) == "" {
			if len(out) > 0 {
				break
			}
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// backlog markers (SPEC §13.3): symbols and words, their status, in precedence order.
var backlogDropped = regexp.MustCompile(`(?i)\b(SUPERSEDED|MOOT|RETIRED)\b`)
var backlogBlocked = regexp.MustCompile(`⏸|(?i)\bBLOCKED\b`)
var backlogDone = regexp.MustCompile(`✓|✅|(?i)\b(DONE|FIXED|CLOSED|RESOLVED)\b`)

// backlogStatus returns the status a backlog heading's markers imply, or "".
func backlogStatus(text string) string {
	switch {
	case struck(text), backlogDropped.MatchString(text):
		return "dropped"
	case backlogBlocked.MatchString(text):
		return "blocked"
	case backlogDone.MatchString(text):
		return "done"
	}
	return ""
}

// leadingMarkers removes marker symbols and marker words from the start of a heading, and
// returns the rest and what was removed.
var leadingMarkerRe = regexp.MustCompile(`^(?:(?:✓|✅|⏸|📋|(?i:DONE|FIXED|CLOSED|RESOLVED|SUPERSEDED|MOOT|RETIRED|BLOCKED))\s*)+`)

func leadingMarkers(text string) (rest, removed string) {
	if struck(text) {
		t := strings.TrimSpace(text)
		text = strings.TrimSpace(t[2 : len(t)-2])
	}
	m := leadingMarkerRe.FindString(text)
	return strings.TrimSpace(text[len(m):]), m
}
