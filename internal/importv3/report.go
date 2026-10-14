package importv3

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arrowheadmind/memex/internal/jsonx"
)

type dupRec struct{ Source, DuplicateOf string }
type tagRec struct{ From, To string }
type rewriteRec struct{ Entry, From, To string }
type countRec struct {
	Target string
	Count  int
}

type srcRec struct {
	Rev, Commit, Basename string
	Primary               bool
}

type fileSummary struct {
	Source, Path string
	Lines, Chars int
	LinesOut     map[string]int
	CharsOut     map[string]int
	Units        int
}

type entryRec struct {
	ID, Type, Kind, Source, Path string
	Status                       string
	Lines                        [2]int
	Removed, Extra               []string
}

type attachRec struct{ Source, Path, Dest string }

type indexReport struct {
	Rows, Matched, Placeholders int
	Orphan, Duplicate           []map[string]string
	Conflict                    []string
}

type linksReport struct {
	Rewritten  []rewriteRec
	WikiQuoted []rewriteRec
	Unresolved []countRec
}

// Report is DIR/import-report.json (SPEC §13.6).
type Report struct {
	Today          string
	Sources        []srcRec
	Files          []fileSummary
	Entries        []entryRec
	Attachments    []attachRec
	NotKnowledge   []string
	Index          indexReport
	TagsNormalized []tagRec
	Links          linksReport
	Duplicates     []dupRec
	AliasDropped   []string
	RowShape       []string
	ShortBody      []string
	DatedByBlame   []string
	MarkersRemoved []rewriteRec
	Leaks          []string
	Warnings       []countRec
	Problems       []string
	Delta          *DeltaReport // set by a delta import only (SPEC §13.7)
}

func strs(ss []string) jsonx.Array { return jsonx.Strings(ss) }

func counts(m map[string]int) jsonx.Object {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var o jsonx.Object
	for _, k := range keys {
		o = append(o, jsonx.Member{Key: k, Value: jsonx.Int(m[k])})
	}
	return o
}

func stringMap(m map[string]string) jsonx.Object {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var o jsonx.Object
	for _, k := range keys {
		o = append(o, jsonx.Member{Key: k, Value: jsonx.String(m[k])})
	}
	return o
}

func countArray(cs []countRec) jsonx.Array {
	a := jsonx.Array{}
	for _, c := range cs {
		a = append(a, jsonx.Object{{Key: "target", Value: jsonx.String(c.Target)}, {Key: "count", Value: jsonx.Int(c.Count)}})
	}
	return a
}

func rewriteArray(rs []rewriteRec) jsonx.Array {
	a := jsonx.Array{}
	for _, r := range rs {
		a = append(a, jsonx.Object{{Key: "entry", Value: jsonx.String(r.Entry)}, {Key: "from", Value: jsonx.String(r.From)}, {Key: "to", Value: jsonx.String(r.To)}})
	}
	return a
}

// JSON renders the report (keys in a fixed order).
func (r *Report) JSON() []byte {
	src := jsonx.Array{}
	for _, s := range r.Sources {
		src = append(src, jsonx.Object{
			{Key: "basename", Value: jsonx.String(s.Basename)}, {Key: "rev", Value: jsonx.String(s.Rev)},
			{Key: "commit", Value: jsonx.String(s.Commit)}, {Key: "primary", Value: jsonx.Bool(s.Primary)},
		})
	}
	files := jsonx.Array{}
	for _, f := range r.Files {
		files = append(files, jsonx.Object{
			{Key: "source", Value: jsonx.String(f.Source)}, {Key: "path", Value: jsonx.String(f.Path)},
			{Key: "lines", Value: jsonx.Int(f.Lines)}, {Key: "chars", Value: jsonx.Int(f.Chars)},
			{Key: "lines_out", Value: counts(f.LinesOut)}, {Key: "chars_out", Value: counts(f.CharsOut)},
			{Key: "units", Value: jsonx.Int(f.Units)},
		})
	}
	entries := jsonx.Array{}
	for _, e := range r.Entries {
		entries = append(entries, jsonx.Object{
			{Key: "id", Value: jsonx.String(e.ID)}, {Key: "type", Value: jsonx.String(e.Type)},
			{Key: "unit", Value: jsonx.String(e.Kind)}, {Key: "source", Value: jsonx.String(e.Source)},
			{Key: "path", Value: jsonx.String(e.Path)}, {Key: "status", Value: jsonx.String(e.Status)},
			{Key: "lines", Value: jsonx.Array{jsonx.Int(e.Lines[0]), jsonx.Int(e.Lines[1])}},
			{Key: "removed", Value: strs(e.Removed)}, {Key: "extra", Value: strs(e.Extra)},
		})
	}
	att := jsonx.Array{}
	for _, a := range r.Attachments {
		att = append(att, jsonx.Object{{Key: "source", Value: jsonx.String(a.Source)}, {Key: "path", Value: jsonx.String(a.Path)}, {Key: "dest", Value: jsonx.String(a.Dest)}})
	}
	maps := func(ms []map[string]string) jsonx.Array {
		a := jsonx.Array{}
		for _, m := range ms {
			a = append(a, stringMap(m))
		}
		return a
	}
	tags := jsonx.Array{}
	for _, t := range r.TagsNormalized {
		tags = append(tags, jsonx.Object{{Key: "from", Value: jsonx.String(t.From)}, {Key: "to", Value: jsonx.String(t.To)}})
	}
	dups := jsonx.Array{}
	for _, d := range r.Duplicates {
		dups = append(dups, jsonx.Object{{Key: "source", Value: jsonx.String(d.Source)}, {Key: "duplicate_of", Value: jsonx.String(d.DuplicateOf)}})
	}
	obj := jsonx.Object{
		{Key: "format", Value: jsonx.Int(1)},
		{Key: "today", Value: jsonx.String(r.Today)},
		{Key: "sources", Value: src},
		{Key: "files", Value: files},
		{Key: "entries", Value: entries},
		{Key: "attachments", Value: att},
		{Key: "not_knowledge", Value: strs(r.NotKnowledge)},
		{Key: "index", Value: jsonx.Object{
			{Key: "rows", Value: jsonx.Int(r.Index.Rows)}, {Key: "matched", Value: jsonx.Int(r.Index.Matched)},
			{Key: "placeholders", Value: jsonx.Int(r.Index.Placeholders)},
			{Key: "orphan", Value: maps(r.Index.Orphan)}, {Key: "duplicate", Value: maps(r.Index.Duplicate)},
			{Key: "conflict", Value: strs(r.Index.Conflict)},
		}},
		{Key: "tags_normalized", Value: tags},
		{Key: "links", Value: jsonx.Object{
			{Key: "rewritten", Value: rewriteArray(r.Links.Rewritten)},
			{Key: "wikilinks_quoted", Value: rewriteArray(r.Links.WikiQuoted)},
			{Key: "unresolved", Value: countArray(r.Links.Unresolved)},
		}},
		{Key: "duplicates", Value: dups},
		{Key: "aliases_dropped", Value: strs(r.AliasDropped)},
		{Key: "row_shape", Value: strs(r.RowShape)},
		{Key: "short_body", Value: strs(r.ShortBody)},
		{Key: "dated_by_blame", Value: strs(r.DatedByBlame)},
		{Key: "markers_removed", Value: rewriteArray(r.MarkersRemoved)},
		{Key: "leaks", Value: strs(r.Leaks)},
		{Key: "warnings", Value: countArray(r.Warnings)},
		{Key: "problems", Value: strs(r.Problems)},
	}
	if r.Delta != nil {
		obj = append(obj, jsonx.Member{Key: "delta", Value: r.Delta.json()})
	}
	return jsonx.Encode(obj)
}

// account fills the per-file summaries and checks every line is attributed and every unit's
// text is preserved (SPEC §13.6).
func (im *Importer) account() {
	for _, f := range im.files {
		fs := fileSummary{Source: f.Src.Basename, Path: f.Path, Lines: len(f.Lines), LinesOut: map[string]int{}, CharsOut: map[string]int{}}
		units := map[int]bool{}
		for i, l := range f.Lines {
			n := len([]rune(nonWS(l)))
			fs.Chars += n
			a := f.Attr[i]
			if a == attrNone {
				im.report.Problems = append(im.report.Problems, fmt.Sprintf("%s:%s:%d unattributed line", f.Src.Basename, f.Path, i+1))
				a = "unattributed"
			}
			fs.LinesOut[a]++
			fs.CharsOut[a] += n
			if a == attrUnit && f.UnitOf[i] >= 0 {
				units[f.UnitOf[i]] = true
			}
		}
		fs.Units = len(units)
		im.report.Files = append(im.report.Files, fs)
	}
	for _, u := range im.units {
		if u.Kind == "dropped" || u.Kind == "attachments" || u.Kind == "leak" || u.Kind == "agent" || len(u.Lines) == 0 {
			continue
		}
		f := im.fileOf(u)
		if f == nil {
			continue
		}
		sorted := append([]int(nil), u.Lines...)
		sort.Ints(sorted)
		// source: the heading text that became the title, with the prefix, date and markers
		// removed at their known positions, then the unit's body lines in order
		var src strings.Builder
		if len(u.TitleLines) > 0 {
			src.WriteString(u.TitleSource + "\n")
		}
		for _, n := range sorted {
			if (f.Attr[n-1] == attrUnit || f.Attr[n-1] == attrRemainder) && f.UnitOf[n-1] == u.Order {
				src.WriteString(f.Lines[n-1] + "\n")
			}
		}
		s := compareText(src.String())
		for _, e := range u.Extra {
			s += compareText(e)
		}
		// output: the title, then the written body with link rewrites mapped back and the
		// generated labels removed where they were put
		var out string
		if u.TitleInText {
			out = compareText(u.Title)
		}
		body := u.Body
		for _, r := range im.report.Links.Rewritten {
			if r.Entry == u.ID {
				body = strings.Replace(body, r.To, r.From, 1)
			}
		}
		for _, label := range u.Labels {
			body = strings.Replace(body, label, "", 1)
		}
		out += compareText(body)
		if u.IndexPrefix != "" {
			out += compareText(strings.TrimPrefix(u.IndexPrefix, "**Index summary:** "))
		}
		if s != out {
			d := firstDiff(s, out)
			im.report.Problems = append(im.report.Problems, fmt.Sprintf("%s: text differs (%s:%s lines %d-%d): source %d chars, output %d chars, at %d: source %q, output %q",
				u.ID, u.Src.Basename, u.File, sorted[0], sorted[len(sorted)-1], len([]rune(s)), len([]rune(out)), d, around(s, d), around(out, d)))
		}
	}
}

func around(s string, at int) string {
	r := []rune(s)
	lo, hi := at-12, at+24
	if lo < 0 {
		lo = 0
	}
	if hi > len(r) {
		hi = len(r)
	}
	if lo > hi {
		lo = hi
	}
	return string(r[lo:hi])
}

func firstDiff(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			return i
		}
	}
	if len(ra) < len(rb) {
		return len(ra)
	}
	return len(rb)
}

func (im *Importer) fileOf(u *Unit) *fileRec {
	for _, f := range im.files {
		if f.Src == u.Src && f.Path == u.File {
			return f
		}
	}
	return nil
}
