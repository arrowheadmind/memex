package commands

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/search"
	"github.com/arrowheadmind/memex/internal/store"
)

// ListFields are the columns --fields selects from (SPEC §11.5), and DefaultListFields the
// text columns without it.
var (
	ListFields        = []string{"created", "type", "status", "id", "path", "title", "summary", "tags"}
	DefaultListFields = []string{"created", "type", "status", "path", "title"}
)

// ListOptions are the options of `memex list` (SPEC §11.5).
type ListOptions struct {
	Filter   search.Filter
	Statuses []string
	Limit    int      // -1 means list_limit; 0 means no limit
	Fields   []string // nil means the default line; set means tab-separated columns
	JSON     bool
}

func created(e *store.Entry) string {
	if d, ok := e.Fields.Date("created"); ok {
		return d.Format(time.DateOnly)
	}
	return ""
}

// column is the text of one --fields column. Tags are joined with commas.
func column(e *store.Entry, name string) string {
	switch name {
	case "created":
		return created(e)
	case "type":
		return e.Type.Name
	case "status":
		return e.Status()
	case "id":
		return e.ID
	case "path":
		return e.Path
	case "title":
		return e.Fields.String("title")
	case "summary":
		return e.Fields.String("summary")
	case "tags":
		return strings.Join(e.Fields.Strings("tags"), ",")
	}
	panic("unknown list field " + name)
}

// List runs `memex list` (SPEC §11.5).
func List(e *Env, o ListOptions) int {
	rs, code, ok := e.loadEntries()
	if !ok {
		return code
	}
	var sel []*store.Entry
	for _, en := range rs.entries {
		if !o.Filter.Pass(en) {
			continue
		}
		switch {
		case len(o.Statuses) > 0:
			if !slices.Contains(o.Statuses, en.Status()) {
				continue
			}
		case !o.Filter.Visible(en):
			continue
		}
		sel = append(sel, en)
	}
	slices.SortStableFunc(sel, func(a, b *store.Entry) int {
		if c := strings.Compare(created(b), created(a)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	limit := o.Limit
	if limit < 0 {
		limit = rs.cfg.ListLimit
	}
	more := 0
	if limit > 0 && len(sel) > limit {
		sel, more = sel[:limit], len(sel)-limit
	}

	if o.JSON {
		items := make(jsonx.Array, len(sel))
		for i, en := range sel {
			summary := jsonx.Null
			if s := en.Fields.String("summary"); s != "" {
				summary = jsonx.String(s)
			}
			items[i] = jsonx.Object{
				{Key: "id", Value: jsonx.String(en.ID)},
				{Key: "type", Value: jsonx.String(en.Type.Name)},
				{Key: "status", Value: jsonx.String(en.Status())},
				{Key: "created", Value: jsonx.String(created(en))},
				{Key: "title", Value: jsonx.String(en.Fields.String("title"))},
				{Key: "summary", Value: summary},
				{Key: "tags", Value: jsonx.Strings(en.Fields.Strings("tags"))},
				{Key: "path", Value: jsonx.String(en.Path)},
			}
		}
		e.Stdout.Write(jsonx.Encode(jsonx.Object{
			{Key: "entries", Value: items},
			{Key: "more", Value: jsonx.Int(more)},
		}))
		return 0
	}
	fields, sep := DefaultListFields, " "
	if o.Fields != nil {
		fields, sep = o.Fields, "\t"
	}
	var b strings.Builder
	for _, en := range sel {
		cols := make([]string, len(fields))
		for i, f := range fields {
			cols[i] = column(en, f)
		}
		b.WriteString(strings.Join(cols, sep) + "\n")
	}
	if more > 0 {
		fmt.Fprintf(&b, "%d more entries not shown (raise --limit)\n", more)
	}
	fmt.Fprint(e.Stdout, b.String())
	return 0
}
