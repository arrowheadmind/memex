package importv3

import (
	"fmt"
	"path"
	"strings"
)

// Attribution categories of a source line (SPEC §13.6).
const (
	attrNone       = ""
	attrUnit       = "unit"
	attrMoved      = "moved-to-frontmatter"
	attrStructural = "structural"
	attrRemainder  = "remainder"
)

// Unit is the smallest piece of source that becomes one entry (SPEC §13.3).
type Unit struct {
	Src  *Source
	File string // source path inside Src
	Kind string // investigation, decision, runbook, plan, prompt, report, doc, km-row, km-section, c-row, c-section, always, task, remainder, attachments, leak

	Type        string
	Title       string
	TitleInText bool // the title's text is not also in the body (heading-derived)
	Date        string
	Status      string
	Summary     string
	Tags        []string
	Repos       []string
	Aliases     []string
	Always      bool
	PlanKind    string
	Verified    string
	Stem        string // filename stem of a file unit, or ""

	Lines       []int    // 1-based source line numbers of the unit (heading and body)
	TitleLines  []int    // 1-based line numbers of the heading that became the title
	BodyLines   []string // body text, source form
	Body        string   // final body
	Removed     []string // text removed from the source on the way to the frontmatter
	Extra       []string // text added to the output from another source line (INDEX summary)
	TitleSource string   // the heading text after the removals, for the §13.6 comparison
	Labels      []string // generated labels in the body (table rows), not text

	StatusValue  string   // status value found in the file, "" if none
	IndexStatus  string   // status value of the matching INDEX row, "" if none
	SupHints     []string // lines searched for a "superseded by" reference
	SupersededBy string
	IndexPrefix  string // "**Index summary:** …" paragraph prepended to the body
	ID           string
	Order        int
	Secondary    bool
}

// fileRec holds a recognized source file and the attribution of each line.
type fileRec struct {
	Src    *Source
	Path   string
	Lines  []string
	Attr   []string // per line, 0-based
	UnitOf []int    // per line, index into the importer's units, -1 if none
}

func (f *fileRec) set(i int, attr string, unit int) {
	f.Attr[i] = attr
	f.UnitOf[i] = unit
}

// splitLines splits text into lines without a final empty element.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func newFileRec(src *Source, p string) (*fileRec, error) {
	text, err := src.Read(p)
	if err != nil {
		return nil, err
	}
	lines := splitLines(text)
	f := &fileRec{Src: src, Path: p, Lines: lines, Attr: make([]string, len(lines)), UnitOf: make([]int, len(lines))}
	for i := range f.UnitOf {
		f.UnitOf[i] = -1
	}
	return f, nil
}

// add appends a unit and attributes its lines: body lines get attrUnit, title lines (the
// heading that became the title) and other moved lines get attrMoved. Title lines take part
// in the §13.6 comparison through their heading text; other moved lines do not.
func (im *Importer) add(f *fileRec, u *Unit, bodyIdx, titleIdx, movedIdx []int) *Unit {
	u.Src, u.File = f.Src, f.Path
	u.Secondary = !f.Src.Primary
	u.Order = len(im.units)
	idx := len(im.units)
	im.units = append(im.units, u)
	for _, i := range bodyIdx {
		f.set(i, attrUnit, idx)
		u.Lines = append(u.Lines, i+1)
		u.BodyLines = append(u.BodyLines, f.Lines[i])
	}
	for _, i := range titleIdx {
		f.set(i, attrMoved, idx)
		u.Lines = append(u.Lines, i+1)
		u.TitleLines = append(u.TitleLines, i+1)
	}
	for _, i := range movedIdx {
		f.set(i, attrMoved, idx)
		u.Lines = append(u.Lines, i+1)
	}
	return u
}

// headerSections are the brainctl-written investigation header sections (SPEC §13.3).
var headerSections = map[string]bool{"Status": true, "Source Repo": true, "Context": true}

// extractFile handles a one-file unit: investigation, decision, runbook, plan, prompt,
// report, doc. The first ATX heading becomes the title and is moved to the frontmatter.
func (im *Importer) extractFile(f *fileRec, kind, typ string) *Unit {
	u := &Unit{Kind: kind, Type: typ, Stem: stem(f.Path), TitleInText: true}
	title := -1
	for i, l := range f.Lines {
		if lvl, _ := heading(l); lvl > 0 {
			title = i
			break
		}
	}
	var moved, titleIdx []int
	consumed := map[int]bool{}
	if title >= 0 {
		_, text := heading(f.Lines[title])
		text, removed := stripTitlePrefix(text)
		if removed != "" {
			u.Removed = append(u.Removed, removed)
		}
		text, date, rem := headingDate(text)
		if rem != "" {
			u.Removed = append(u.Removed, rem)
		}
		u.Date = date
		u.TitleSource = text
		u.Title = cutTitle(stripMarkup(text))
		if u.Title != strings.Join(strings.Fields(stripMarkup(text)), " ") {
			u.TitleInText = false // cut: the full heading stays in the body, nothing is removed
			u.Removed = nil
		}
		titleIdx = []int{title}
		consumed[title] = true
		if kind == "investigation" {
			im.extractHeader(f, u, title, consumed, &moved)
		}
	} else {
		// no heading: a leading YAML block's name and description, else the filename
		name, desc := yamlBlock(f.Lines)
		if name != "" {
			u.Title = cutTitle(stripMarkup(name))
			u.Summary = desc
		} else {
			u.Title = cutTitle(u.Stem)
		}
		u.TitleInText = false
	}
	if u.Title == "" {
		u.Title = cutTitle(u.Stem)
	}
	if !u.TitleInText && title >= 0 {
		// the full heading text must survive somewhere: it stays as the first body line
		consumed[title] = false
		titleIdx = nil
	}
	var body []int
	for i := range f.Lines {
		if consumed[i] {
			continue
		}
		if f.Attr[i] == attrStructural {
			continue
		}
		body = append(body, i)
	}
	if m := filenameDate.FindString(path.Base(f.Path)); m != "" {
		u.Date = m
	}
	// status value of plans and decisions: "## Status" section or "**Status:**" line
	if kind == "plan" || kind == "decision" || kind == "investigation" && u.StatusValue == "" {
		im.findStatus(f, u, consumed)
	}
	// blockquotes mentioning "supersed" are superseded-callout candidates (decisions and
	// other file units; an investigation's hints are its Status section only)
	var quote []string
	for i, l := range f.Lines {
		if kind == "investigation" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(l), ">") {
			quote = append(quote, l)
			if i+1 < len(f.Lines) && strings.HasPrefix(strings.TrimSpace(f.Lines[i+1]), ">") {
				continue
			}
			joined := strings.Join(quote, "\n")
			if strings.Contains(strings.ToLower(joined), "supersed") {
				u.SupHints = append(u.SupHints, quote...)
			}
			quote = nil
		} else {
			quote = nil
		}
	}
	return im.add(f, u, body, titleIdx, moved)
}

// extractHeader removes the brainctl header of an investigation: the Status, Source Repo
// and Context sections directly after the title, each at most once. A section's value line
// moves to the frontmatter only when it is that value alone (a single token); a line that
// says more stays in the body (SPEC §13.3). The Status section's lines are the only
// "superseded by" hints of an investigation.
func (im *Importer) extractHeader(f *fileRec, u *Unit, title int, consumed map[int]bool, moved *[]int) {
	i := title + 1
	seen := map[string]bool{}
	keptLine := false // a value line stayed in the body: the blank lines after it stay too
	for i < len(f.Lines) {
		if strings.TrimSpace(f.Lines[i]) == "" {
			if !keptLine {
				f.Attr[i] = attrStructural
			}
			i++
			continue
		}
		lvl, text := heading(f.Lines[i])
		if lvl != 2 || !headerSections[text] || seen[text] {
			break
		}
		keptLine = false
		seen[text] = true
		consumed[i] = true
		*moved = append(*moved, i)
		// the value: the first non-blank line before the next heading
		j := i + 1
		for j < len(f.Lines) && strings.TrimSpace(f.Lines[j]) == "" {
			f.Attr[j] = attrStructural
			j++
		}
		if j < len(f.Lines) {
			if l, _ := heading(f.Lines[j]); l == 0 {
				value := strings.TrimSpace(f.Lines[j])
				alone := !strings.ContainsAny(value, " \t,")
				switch text {
				case "Status":
					u.StatusValue = statusValue(value)
					for k := j; k < len(f.Lines); k++ {
						if lv, _ := heading(f.Lines[k]); lv > 0 {
							break
						}
						if strings.TrimSpace(f.Lines[k]) != "" {
							u.SupHints = append(u.SupHints, f.Lines[k])
						}
					}
				case "Source Repo":
					if fs := strings.Fields(value); len(fs) > 0 {
						u.Repos = []string{fs[0]}
					}
				case "Context":
					for _, t := range strings.Split(value, ",") {
						if n := tagify(t); n != "" && !contextNames[n] {
							u.Tags = append(u.Tags, n)
						}
					}
				}
				if alone {
					consumed[j] = true
					*moved = append(*moved, j)
				} else {
					keptLine = true
				}
				j++
			}
		}
		i = j
	}
}

// findStatus looks for a "## Status" section or a "**Status:**" line that stays in the body.
func (im *Importer) findStatus(f *fileRec, u *Unit, consumed map[int]bool) {
	code := fenced(f.Lines)
	for i, l := range f.Lines {
		if consumed[i] || code[i] {
			continue
		}
		if lvl, text := heading(l); lvl == 2 && text == "Status" {
			for j := i + 1; j < len(f.Lines); j++ {
				if lv, _ := heading(f.Lines[j]); lv > 0 {
					break
				}
				if strings.TrimSpace(f.Lines[j]) == "" || code[j] {
					continue
				}
				if u.StatusValue == "" {
					u.StatusValue = statusValue(f.Lines[j])
				}
				u.SupHints = append(u.SupHints, f.Lines[j])
			}
			if u.StatusValue != "" {
				return
			}
		}
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "**Status:**") || strings.HasPrefix(t, "**Status**:") {
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "**Status:**"), "**Status**:"))
			if u.StatusValue == "" {
				u.StatusValue = statusValue(rest)
			}
			u.SupHints = append(u.SupHints, rest)
			return
		}
	}
}

// sectionRange returns, for a "## " heading at i, the index after the heading lines (joined
// hard-wrapped headings) and the joined text.
func sectionHeading(lines []string, i int) (next int, text string) {
	_, text = heading(lines[i])
	next = i + 1
	for next < len(lines) {
		lvl, t := heading(lines[next])
		if lvl != 2 || strings.TrimSpace(lines[next-1]) == "" {
			break
		}
		text += " " + t
		next++
	}
	return next, text
}

// extractRules handles known-mistakes.md and constraints.md (SPEC §13.3, §13.4): the
// top-level table before the first "## ", then one rule per "## " section, with "## Always"
// bullets as always rules in constraints.md.
func (im *Importer) extractRules(f *fileRec, isConstraints bool) {
	lines := f.Lines
	firstSection := len(lines)
	for i, l := range lines {
		if lvl, _ := heading(l); lvl == 2 {
			firstSection = i
			break
		}
	}
	// top-level file title
	if len(lines) > 0 {
		if lvl, _ := heading(lines[0]); lvl == 1 {
			f.Attr[0] = attrStructural
		}
	}
	// the table region
	var header []string
	for i := 0; i < firstSection; i++ {
		if f.Attr[i] != attrNone {
			continue
		}
		l := lines[i]
		switch {
		case strings.TrimSpace(l) == "":
			f.Attr[i] = attrStructural
		case isTableRow(l) && tableSepRe.MatchString(l):
			f.Attr[i] = attrStructural
		case isTableRow(l) && i+1 < firstSection && tableSepRe.MatchString(lines[i+1]):
			header = splitCells(l)
			f.Attr[i] = attrStructural
		case isTableRow(l) && header != nil:
			im.addRow(f, i, header, isConstraints)
		default:
			f.Attr[i] = attrRemainder
		}
	}
	// sections
	i := firstSection
	for i < len(lines) {
		if lvl, _ := heading(lines[i]); lvl != 2 {
			i++
			continue
		}
		next, text := sectionHeading(lines, i)
		end := next
		for end < len(lines) {
			if lvl, _ := heading(lines[end]); lvl == 2 {
				break
			}
			end++
		}
		if isConstraints && text == "Always" {
			for k := i; k < next; k++ {
				f.Attr[k] = attrStructural
			}
			im.extractAlways(f, next, end)
			i = end
			continue
		}
		content := false
		for k := next; k < end; k++ {
			if strings.TrimSpace(lines[k]) != "" {
				content = true
				break
			}
		}
		if !content {
			for k := i; k < end; k++ {
				f.Attr[k] = attrStructural
			}
			i = end
			continue
		}
		u := &Unit{Kind: "section", Type: "rule", Status: "active", TitleInText: true}
		if isConstraints {
			u.Kind = "c-section"
		} else {
			u.Kind = "km-section"
		}
		rest, tag, removed, prefixDate := stripPrefix(text)
		if removed != "" {
			u.Removed = append(u.Removed, removed)
			u.Tags = append(u.Tags, tag)
		}
		rest, date, rem := headingDate(rest)
		if rem != "" {
			u.Removed = append(u.Removed, rem)
		}
		if date == "" {
			date = prefixDate
		}
		u.Date = date
		u.TitleSource = rest
		full := stripMarkup(rest)
		u.Title = cutTitle(full)
		if struck(rest) {
			u.Tags = append(u.Tags, "marker-struck")
		}
		var titleIdx, body []int
		for k := i; k < next; k++ {
			titleIdx = append(titleIdx, k)
		}
		if u.Title != strings.Join(strings.Fields(full), " ") {
			// cut title: the heading stays in the body so no text is lost
			u.TitleInText = false
			u.Removed = nil
			body = append(body, titleIdx...)
			titleIdx = nil
		}
		for k := next; k < end; k++ {
			body = append(body, k)
		}
		im.add(f, u, body, titleIdx, nil)
		i = end
	}
	for i := range lines {
		if f.Attr[i] == attrNone {
			f.Attr[i] = attrRemainder
		}
	}
}

// addRow makes a rule from a table row.
func (im *Importer) addRow(f *fileRec, i int, header []string, isConstraints bool) {
	cells := splitCells(f.Lines[i])
	shape := false
	if len(cells) > len(header) {
		extra := strings.Join(cells[len(header)-1:], " | ")
		cells = append(cells[:len(header)-1], extra)
		shape = true
	}
	u := &Unit{Kind: "km-row", Type: "rule", Status: "active"}
	if isConstraints {
		u.Kind = "c-row"
	}
	first := ""
	if len(cells) > 0 {
		first = unescapeCell(cells[0])
	}
	u.Title = cutTitle(stripMarkup(first))
	var paras []string
	for k, c := range cells {
		label := "**" + header[k] + ":** "
		paras = append(paras, label+unescapeCell(c))
		u.Labels = append(u.Labels, label)
	}
	im.add(f, u, []int{i}, nil, nil)
	// the row line is the source; the body is generated from its cells
	u.BodyLines = []string{strings.Join(paras, "\n\n")}
	if shape {
		im.report.RowShape = append(im.report.RowShape, fmt.Sprintf("%s:%d", f.Path, i+1))
	}
	if struck(first) {
		u.Tags = append(u.Tags, "marker-struck")
	}
}

// extractAlways makes one always rule per top-level bullet of the "## Always" section.
func (im *Importer) extractAlways(f *fileRec, start, end int) {
	lines := f.Lines
	i := start
	n := 0
	for i < end {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			f.Attr[i] = attrStructural
			i++
			continue
		}
		if !(strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")) {
			f.Attr[i] = attrRemainder
			i++
			continue
		}
		j := i + 1
		last := i
		for j < end {
			t := lines[j]
			if strings.TrimSpace(t) == "" {
				j++
				continue
			}
			if !strings.HasPrefix(t, " ") && !strings.HasPrefix(t, "\t") {
				break
			}
			last = j
			j++
		}
		n++
		u := &Unit{Kind: "always", Type: "rule", Status: "active", Always: true}
		text := strings.TrimSpace(l[2:])
		joined := text
		for k := i + 1; k <= last; k++ {
			joined += " " + strings.TrimSpace(lines[k])
		}
		var title string
		if strings.HasPrefix(joined, "**") {
			if e := strings.Index(joined[2:], "**"); e >= 0 {
				title = joined[2 : 2+e]
			}
		}
		if title == "" {
			title = joined
			for _, sep := range []string{". ", ": ", " — "} {
				if k := strings.Index(title, sep); k >= 0 {
					title = title[:k]
				}
			}
		}
		u.Title = cutTitle(stripMarkup(title))
		u.Aliases = []string{fmt.Sprintf("%s#always-%d", f.Path, n)}
		var body []int
		for k := i; k <= last; k++ {
			body = append(body, k)
		}
		for k := last + 1; k < j; k++ {
			f.Attr[k] = attrStructural
		}
		im.add(f, u, body, nil, nil)
		i = j
	}
}

// extractBacklog makes one task per "### " item of a backlog file (SPEC §13.3).
func (im *Importer) extractBacklog(f *fileRec) {
	lines := f.Lines
	section := ""
	i := 0
	for i < len(lines) {
		l := lines[i]
		lvl, text := heading(l)
		switch {
		case lvl == 1 && i == 0:
			f.Attr[i] = attrStructural
			i++
		case lvl == 2:
			section = text
			f.Attr[i] = attrStructural
			i++
		case lvl == 3:
			end := i + 1
			for end < len(lines) {
				if lv, _ := heading(lines[end]); lv == 2 || lv == 3 {
					break
				}
				end++
			}
			u := &Unit{Kind: "task", Type: "task", TitleInText: true}
			switch section {
			case "In Progress":
				u.Status = "in-progress"
			case "To Do":
				u.Status = "open"
			case "Done":
				u.Status = "done"
			default:
				u.Status = "open"
			}
			st, rest, marker := headingMarker(text)
			if st != "" {
				u.Status = st
			}
			if marker != "" {
				u.Removed = append(u.Removed, marker)
				im.report.MarkersRemoved = append(im.report.MarkersRemoved, rewriteRec{Entry: fmt.Sprintf("%s:%d", f.Path, i+1), From: marker, To: st})
			}
			rest, date, rem := headingDate(rest)
			if rem != "" {
				u.Removed = append(u.Removed, rem)
			}
			if marker != "" {
				rest = trimSeparators(rest) // the separator that joined the marker goes with it
			}
			u.Date = date
			u.TitleSource = rest
			full := stripMarkup(rest)
			u.Title = cutTitle(full)
			var titleIdx, body []int
			titleIdx = []int{i}
			if u.Title != strings.Join(strings.Fields(full), " ") {
				u.TitleInText = false
				u.Removed = nil
				body = append(body, i)
				titleIdx = nil
			}
			for k := i + 1; k < end; k++ {
				body = append(body, k)
			}
			im.add(f, u, body, titleIdx, nil)
			i = end
		default:
			if strings.TrimSpace(l) == "" {
				f.Attr[i] = attrStructural
			} else {
				f.Attr[i] = attrRemainder
			}
			i++
		}
	}
}
