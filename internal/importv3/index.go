package importv3

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// MaxSummary is the summary length cap (SPEC §5.3).
const MaxSummary = 500

// contextNames are brainctl's own BRAIN_CONTEXT values, which an INDEX Context cell carries
// instead of a topic; they are placeholders, not tags (SPEC §13.3).
var contextNames = map[string]bool{"root": true, "dev": true, "unknown": true}

// indexRow is one row of INDEX.md.
type indexRow struct {
	Line                                         int
	Date, File, Summary, Tags, Context, Repo, St string
}

// extractIndex reads a source's INDEX.md, matches rows to units and fills their
// frontmatter (SPEC §13.3). Rows that match nothing stay in the file's remainder.
func (im *Importer) extractIndex(f *fileRec) {
	lines := f.Lines
	var header []string
	matched := map[*Unit]*indexRow{}
	rowOf := map[*Unit]int{}
	for i, l := range lines {
		switch {
		case i == 0 && func() bool { lvl, _ := heading(l); return lvl == 1 }():
			f.Attr[i] = attrStructural
		case strings.TrimSpace(l) == "":
			f.Attr[i] = attrStructural
		case isTableRow(l) && tableSepRe.MatchString(l):
			f.Attr[i] = attrStructural
		case isTableRow(l) && i+1 < len(lines) && tableSepRe.MatchString(lines[i+1]):
			header = splitCells(l)
			f.Attr[i] = attrStructural
		case isTableRow(l) && header != nil:
			cells := splitCells(l)
			get := func(name string) string {
				for k, h := range header {
					if strings.EqualFold(strings.TrimSpace(h), name) && k < len(cells) {
						return unescapeCell(cells[k])
					}
				}
				return ""
			}
			row := &indexRow{Line: i + 1, Date: get("Date"), File: get("File"), Summary: get("Summary"),
				Tags: get("Tags"), Context: get("Context"), Repo: get("Repo"), St: get("Status")}
			im.report.Index.Rows++
			u := im.matchIndexFile(f.Src, row.File)
			if u == nil {
				im.report.Index.Orphan = append(im.report.Index.Orphan, indexRec(f, row))
				f.Attr[i] = attrRemainder
				continue
			}
			if prev, ok := matched[u]; ok {
				im.report.Index.Duplicate = append(im.report.Index.Duplicate, indexRec(f, prev))
				f.Attr[rowOf[u]] = attrRemainder
			}
			matched[u] = row
			rowOf[u] = i
			f.Attr[i] = attrMoved
			f.UnitOf[i] = u.Order
		default:
			f.Attr[i] = attrRemainder
		}
	}
	// apply in row order, so that the report is deterministic
	var us []*Unit
	for u := range matched {
		us = append(us, u)
	}
	sort.Slice(us, func(i, j int) bool { return matched[us[i]].Line < matched[us[j]].Line })
	for _, u := range us {
		im.applyIndexRow(u, matched[u], f)
	}
}

// matchIndexFile resolves an INDEX File cell to a unit.
func (im *Importer) matchIndexFile(src *Source, file string) *Unit {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil
	}
	if !strings.Contains(file, "/") {
		return im.unitAt(src, ".brain/investigations/"+file)
	}
	if u := im.resolveRef(src, file); u != nil {
		return u
	}
	return nil
}

// applyIndexRow fills a unit from its row: summary and tags only when the file has none,
// status only when the file has no status value, placeholders ignored (SPEC §13.3).
func (im *Importer) applyIndexRow(u *Unit, row *indexRow, f *fileRec) {
	im.report.Index.Matched++
	summary := strings.TrimSpace(row.Summary)
	if summary == "manual-summary" {
		summary = ""
		im.report.Index.Placeholders++
	}
	if summary != "" && u.Summary == "" {
		if utf8.RuneCountInString(summary) > MaxSummary {
			r := []rune(summary)
			cut := string(r[:MaxSummary])
			if k := strings.LastIndex(cut, " "); k > 0 {
				cut = cut[:k]
			}
			u.Summary = strings.TrimSpace(cut)
			u.IndexPrefix = "**Index summary:** " + summary
			u.Extra = append(u.Extra, summary)
		} else {
			u.Summary = summary
		}
	}
	headerTags := len(u.Tags) > 0 // the file's own Context line gave tags: the row's do not apply
	for ci, cell := range []string{row.Tags, row.Context} {
		if headerTags {
			break
		}
		cell = strings.TrimSpace(cell)
		if cell == "manual" {
			im.report.Index.Placeholders++
			continue
		}
		for _, t := range strings.Split(cell, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			n := tagify(t)
			if n == "" {
				continue
			}
			if ci == 1 && contextNames[n] {
				im.report.Index.Placeholders++
				continue
			}
			if n != t {
				im.report.TagsNormalized = append(im.report.TagsNormalized, tagRec{From: t, To: n})
			}
			u.Tags = append(u.Tags, n)
		}
	}
	if repo := tagify(row.Repo); repo != "" && !im.emptyRepo(repo) && len(u.Repos) == 0 {
		u.Repos = []string{repo}
	}
	st := statusValue(row.St)
	if st != "" {
		u.IndexStatus = st
		if u.StatusValue != "" && u.StatusValue != st {
			im.report.Index.Conflict = append(im.report.Index.Conflict,
				fmt.Sprintf("%s:%d %s: file %q, index %q", f.Path, row.Line, u.File, u.StatusValue, st))
		}
	}
}

func indexRec(f *fileRec, row *indexRow) map[string]string {
	return map[string]string{
		"source": f.Src.Basename + ":" + f.Path, "line": fmt.Sprint(row.Line), "date": row.Date,
		"file": row.File, "summary": row.Summary, "tags": row.Tags, "context": row.Context,
		"repo": row.Repo, "status": row.St,
	}
}
