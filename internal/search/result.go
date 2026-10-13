package search

import (
	"slices"

	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// Filter is the filter and visibility options of SPEC §8.4 (and §11.5). Values within one
// option are OR'd; different options are AND'd; an empty option does not filter.
type Filter struct {
	Types              []*spec.Type
	Tags, Repos, Hosts []string
	All                bool // every status is visible
}

// Pass reports whether e passes the --type, --tag, --repo and --host filters.
func (f *Filter) Pass(e *store.Entry) bool {
	if len(f.Types) > 0 && !slices.ContainsFunc(f.Types, func(t *spec.Type) bool { return t.Name == e.Type.Name }) {
		return false
	}
	return anyOf(f.Tags, e.Fields.Strings("tags")) &&
		anyOf(f.Repos, e.Fields.Strings("repos")) &&
		anyOf(f.Hosts, e.Fields.Strings("hosts"))
}

// Visible reports whether e is visible: its status is in the type's visible-by-default set,
// or --all was given (SPEC §8.4).
func (f *Filter) Visible(e *store.Entry) bool { return f.All || e.Visible() }

func anyOf(want, have []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		if slices.Contains(have, w) {
			return true
		}
	}
	return false
}

// Result is a search sorted into the groups of SPEC §8.4.
type Result struct {
	Query
	Shown                  []Hit
	More, Hidden, Excluded int
}

// Matched reports whether any document matched any item.
func (r *Result) Matched() bool {
	return len(r.Shown)+r.More+r.Hidden+r.Excluded > 0
}

// ExitCode is SPEC §8.5's: 3 when nothing is shown, else 0.
func (r *Result) ExitCode() int {
	if len(r.Shown) == 0 {
		return spec.ExitNoMatch
	}
	return spec.ExitOK
}

// Run searches the corpus and sorts the ranked hits into shown, more, hidden and excluded.
// limit 0 means no limit.
func (c *Corpus) Run(q Query, f *Filter, limit int) *Result {
	r := &Result{Query: q}
	for _, h := range c.Search(q.Items) {
		switch {
		case !f.Pass(h.Entry):
			r.Excluded++
		case !f.Visible(h.Entry):
			r.Hidden++
		case limit == 0 || len(r.Shown) < limit:
			r.Shown = append(r.Shown, h)
		default:
			r.More++
		}
	}
	return r
}
