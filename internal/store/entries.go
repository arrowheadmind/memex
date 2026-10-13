package store

import (
	"github.com/arrowheadmind/memex/internal/entry"
)

// Entry is one entry file read for the read commands (search, list, show, preflight).
type Entry struct {
	File
	Data   []byte       // the file, verbatim
	Fields entry.Fields // the well-formed frontmatter fields
	Body   string
}

// Status returns the entry's status, or "" when it has none.
func (e *Entry) Status() string { return e.Fields.String("status") }

// Visible reports whether the entry's status is in its type's visible-by-default set
// (SPEC §5.2).
func (e *Entry) Visible() bool {
	st := e.Status()
	for _, v := range e.Type.Visible {
		if v == st {
			return true
		}
	}
	return false
}

// Skipped is an entry file the read commands could not use, and why.
type Skipped struct {
	Path    string
	Problem string
}

// LoadEntries reads the manifest and every entry of t, in path order. An entry whose bytes
// are not valid (E-ENCODING) or whose frontmatter cannot be read as TOML (E-FRONTMATTER) has
// no usable fields and is returned in skipped instead. Other field problems leave the
// well-formed fields usable, as in check. A manifest format this memex cannot read is a
// *FormatError.
func LoadEntries(t Tree) (entries []*Entry, skipped []Skipped, err error) {
	if _, _, err := ReadManifest(t); err != nil {
		return nil, nil, err
	}
	layout, err := Enumerate(t)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range layout.Files {
		data, err := t.ReadFile(f.Path)
		if err != nil {
			return nil, nil, err
		}
		if ps := entry.CheckEncoding(data); len(ps) > 0 {
			skipped = append(skipped, Skipped{f.Path, ps[0].Message})
			continue
		}
		split, p := entry.SplitFile(string(data))
		if p != nil {
			skipped = append(skipped, Skipped{f.Path, p.Message})
			continue
		}
		fields, ps := entry.ParseFields(split, f.Type, f.ID)
		if fields == nil {
			skipped = append(skipped, Skipped{f.Path, ps[0].Message})
			continue
		}
		entries = append(entries, &Entry{File: f, Data: data, Fields: fields, Body: split.Body})
	}
	return entries, skipped, nil
}
