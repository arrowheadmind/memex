package commands

import (
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/links"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// ShowOptions are the options of `memex show` (SPEC §11.6).
type ShowOptions struct {
	IDs  []string
	JSON bool
}

// entryLinks are the five link lists of SPEC §11.6. linksTo is in body order; the others are
// in path order.
type entryLinks struct {
	citedBy, supersedes, children, linksTo, linkedFrom []*store.Entry
}

// bodyTargets resolves the [[...]] links of a body (SPEC §6): an id, else the alias of
// exactly one entry. Unresolved targets are left out; each entry appears once, first
// occurrence first.
func (r *resolver) bodyTargets(body string) []*store.Entry {
	refs, _ := links.Extract(body)
	var out []*store.Entry
	for _, ref := range refs {
		t := r.byID[ref.Target]
		if t == nil {
			if hs := r.byAlias[ref.Target]; len(hs) == 1 {
				t = hs[0]
			}
		}
		if t != nil && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func (r *resolver) linksOf(en *store.Entry) entryLinks {
	l := entryLinks{linksTo: r.bodyTargets(en.Body)}
	for _, o := range r.entries { // path order
		f := o.Fields
		if slices.Contains(f.Strings("sources"), en.ID) {
			l.citedBy = append(l.citedBy, o)
		}
		if f.String("superseded_by") == en.ID {
			l.supersedes = append(l.supersedes, o)
		}
		if f.String("parent") == en.ID {
			l.children = append(l.children, o)
		}
		if slices.Contains(r.bodyTargets(o.Body), en) {
			l.linkedFrom = append(l.linkedFrom, o)
		}
	}
	return l
}

// linkList is one line of the links section: its text label, its JSON key and its entries.
type linkList struct {
	label, key string
	entries    []*store.Entry
}

func (l entryLinks) lists() []linkList {
	return []linkList{
		{"cited by", "cited_by", l.citedBy},
		{"supersedes", "supersedes", l.supersedes},
		{"children", "children", l.children},
		{"links to", "links_to", l.linksTo},
		{"linked from", "linked_from", l.linkedFrom},
	}
}

// frontmatterJSON is the §5.3 fields present, in canonical order, dates as strings.
func frontmatterJSON(f entry.Fields) jsonx.Object {
	obj := jsonx.Object{}
	for _, def := range spec.Fields {
		var v jsonx.Value
		switch x := f[def.Name].(type) {
		case string:
			v = jsonx.String(x)
		case time.Time:
			v = jsonx.String(x.Format(time.DateOnly))
		case int64:
			v = jsonx.Int(x)
		case bool:
			v = jsonx.Bool(x)
		case []string:
			v = jsonx.Strings(x)
		default: // absent, or present but invalid
			continue
		}
		obj = append(obj, jsonx.Member{Key: def.Name, Value: v})
	}
	return obj
}

// Show runs `memex show` (SPEC §11.6). Unknown ids are reported on standard error and exit 1,
// after the known ones are printed.
func Show(e *Env, o ShowOptions) int {
	rs, code, ok := e.loadEntries()
	if !ok {
		return code
	}
	r := newResolver(rs.entries)
	var found []*store.Entry
	code = spec.ExitOK
	for _, id := range o.IDs {
		if en := r.resolve(e, id); en != nil {
			found = append(found, en)
		} else {
			code = spec.ExitFailure
		}
	}
	if o.JSON {
		items := make(jsonx.Array, len(found))
		for i, en := range found {
			var ls jsonx.Object
			for _, l := range r.linksOf(en).lists() {
				ids := make([]string, len(l.entries))
				for j, x := range l.entries {
					ids[j] = x.ID
				}
				ls = append(ls, jsonx.Member{Key: l.key, Value: jsonx.Strings(ids)})
			}
			items[i] = jsonx.Object{
				{Key: "id", Value: jsonx.String(en.ID)},
				{Key: "type", Value: jsonx.String(en.Type.Name)},
				{Key: "path", Value: jsonx.String(en.Path)},
				{Key: "frontmatter", Value: frontmatterJSON(en.Fields)},
				{Key: "body", Value: jsonx.String(en.Body)},
				{Key: "links", Value: ls},
			}
		}
		e.Stdout.Write(jsonx.Encode(jsonx.Object{{Key: "entries", Value: items}}))
		return code
	}
	var b strings.Builder
	for i, en := range found {
		if i > 0 {
			b.WriteString("---\n")
		}
		b.Write(en.Data)
		if len(en.Data) > 0 && en.Data[len(en.Data)-1] != '\n' {
			b.WriteString("\n")
		}
		b.WriteString("\n--- links\n")
		for _, l := range r.linksOf(en).lists() {
			if len(l.entries) == 0 {
				continue
			}
			paths := make([]string, len(l.entries))
			for j, x := range l.entries {
				paths[j] = x.Path
			}
			b.WriteString(l.label + ": " + strings.Join(paths, ", ") + "\n")
		}
	}
	e.Stdout.Write([]byte(b.String()))
	return code
}
