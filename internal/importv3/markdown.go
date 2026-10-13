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
	dashDateRe   = regexp.MustCompile(`\s+—\s+(\d{4}-\d{2}-\d{2})\s*$`)
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

// headingDate applies §13.3's date rule to a heading: a trailing "(YYYY-MM-DD)",
// ", found YYYY-MM-DD" or " — YYYY-MM-DD" is removed from the title; the unit's date is the
// EARLIEST date written anywhere in the heading. removed holds the text taken out.
func headingDate(text string) (title, date, removed string) {
	title = text
	for _, re := range []*regexp.Regexp{trailDateRe, foundDateRe, dashDateRe} {
		if m := re.FindStringSubmatchIndex(text); m != nil {
			title, removed = strings.TrimSpace(text[:m[0]]), text[m[0]:]
			break
		}
	}
	for _, d := range dateRe.FindAllString(text, -1) {
		if date == "" || d < date {
			date = d
		}
	}
	return title, date, removed
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

var historicalRe = regexp.MustCompile(`^Historical mistake(\s*\(([^)]*)\))?:\s*`)

// stripPrefix removes a §13.3 heading prefix; it returns the rest, the tag, the removed text
// and, for "Historical mistake (superseded YYYY-MM-DD):", the date in the parenthesis, which
// is the unit's date only when the rest of the heading has none.
func stripPrefix(text string) (rest, tag, removed, date string) {
	if m := historicalRe.FindStringSubmatch(text); m != nil {
		if d := dateRe.FindString(m[2]); d != "" {
			date = d
		}
		return strings.TrimSpace(text[len(m[0]):]), "historical", m[0], date
	}
	for _, p := range prefixTags {
		if p.prefix == "Historical mistake" {
			continue
		}
		if strings.HasPrefix(text, p.prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, p.prefix)), p.tag, p.prefix, ""
		}
	}
	return text, "", "", ""
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

// yamlBlock reads the name: and description: values of a leading YAML block (a first line
// `---`, then lines up to the next `---`), for files that have no heading (SPEC §13.3).
func yamlBlock(lines []string) (name, description string) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", ""
	}
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			break
		}
		if k, v, ok := strings.Cut(l, ":"); ok {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			switch strings.TrimSpace(k) {
			case "name":
				name = v
			case "description":
				description = v
			}
		}
	}
	return name, description
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

// Backlog markers (SPEC §13.3). A marker is one of the listed tokens, case-sensitive, found
// outside code spans: a symbol anywhere (with the marker word right after it, if any), or a
// word at the start of the heading (after an optional "YYYY-MM-DD — " prefix) or as its last
// token. The first marker found decides the status, and only it is removed from the title.
var markerWords = map[string]string{
	"DONE": "done", "FIXED": "done", "CLOSED": "done", "RESOLVED": "done",
	"BLOCKED":    "blocked",
	"SUPERSEDED": "dropped", "MOOT": "dropped", "RETIRED": "dropped",
}
var markerSymbols = map[string]string{"✓": "done", "✅": "done", "⏸": "blocked", "✗": ""}

// headingMarker finds the first marker of a backlog heading. It returns the status it
// implies ("" when there is none), the heading with that marker removed, and the removed
// text. A fully struck-through heading is dropped, and loses its ~~.
func headingMarker(text string) (status, rest, removed string) {
	if struck(text) {
		t := strings.TrimSpace(text)
		return "dropped", strings.TrimSpace(t[2 : len(t)-2]), "~~"
	}
	spans := codeSpans(text)
	type tok struct {
		start, end int
		s          string
	}
	var toks []tok
	i := 0
	for i < len(text) {
		if text[i] == ' ' || text[i] == '\t' {
			i++
			continue
		}
		j := i
		for j < len(text) && text[j] != ' ' && text[j] != '\t' {
			j++
		}
		toks = append(toks, tok{i, j, text[i:j]})
		i = j
	}
	word := func(s string) string { return strings.TrimRight(s, ":;,.") }
	isDate := func(s string) bool { return dateRe.MatchString(s) && len(s) == 10 }
	cut := func(a, b int) (string, string) {
		rest := strings.TrimSpace(text[:a] + " " + text[b:])
		return strings.Join(strings.Fields(rest), " "), text[a:b]
	}
	for k, t := range toks {
		if inSpans(spans, t.start, t.end) {
			continue
		}
		if st, ok := markerSymbols[t.s]; ok {
			// a symbol, with the marker word right after it if there is one
			if k+1 < len(toks) && !inSpans(spans, toks[k+1].start, toks[k+1].end) {
				if ws, ok := markerWords[word(toks[k+1].s)]; ok {
					rest, removed := cut(t.start, toks[k+1].end)
					return ws, rest, removed
				}
			}
			if st == "" { // ✗ alone says nothing
				continue
			}
			rest, removed := cut(t.start, t.end)
			return st, rest, removed
		}
		ws, ok := markerWords[word(t.s)]
		if !ok {
			continue
		}
		atStart := k == 0 || (k == 2 && isDate(toks[0].s) && toks[1].s == "—")
		if atStart || k == len(toks)-1 {
			rest, removed := cut(t.start, t.end)
			return ws, rest, removed
		}
	}
	return "", text, ""
}

// searchKey is the text `git log -S` looks for when dating a line unit (SPEC §13.3): from
// the unit's first line without its ATX marks, or from a table row's first cell, the longest
// piece between marker tokens and dates (which change when an item is closed or re-filed),
// cut to its first 60 code points. Markup stays, because -S matches the file's bytes
// literally and a key that is not in the file dates nothing.
func searchKey(line string) string {
	s := line
	if lvl, text := heading(s); lvl > 0 {
		s = text
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "|") {
		// a table row is identified by its first cell; the other cells get reworded
		if cells := splitCells(s); len(cells) > 0 && len([]rune(cells[0])) >= 4 {
			s = cells[0]
		} else {
			s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(s, "|")), "|"))
		}
	}
	pieces := volatileRe.Split(s, -1)
	best := ""
	for _, p := range pieces {
		p = strings.Trim(p, pieceEdges)
		if len([]rune(p)) > len([]rune(best)) {
			best = p
		}
	}
	if r := []rune(best); len(r) > 60 {
		best = strings.TrimSpace(string(r[:60]))
	}
	return best
}

// pieceEdges are trimmed from both ends of a key piece: the separators and punctuation that
// arrive together with a marker ("— Revisit:" only exists since the marker was added).
const pieceEdges = " \t—-:(),."

// separatorEdges are the separator runs removed from a title's ends after a marker is
// taken out (SPEC §13.3).
const separatorEdges = " \t—-:"

// trimSeparators removes a leading or trailing separator run from a title.
func trimSeparators(title string) string {
	return strings.Join(strings.Fields(strings.Trim(title, separatorEdges)), " ")
}

// volatileRe matches the tokens that change when a backlog item or rule changes state:
// marker symbols and words, and dates.
var volatileRe = regexp.MustCompile(`✓|✅|✗|⏸|\b(DONE|FIXED|CLOSED|RESOLVED|BLOCKED|SUPERSEDED|MOOT|RETIRED)\b|\d{4}-\d{2}-\d{2}`)
