package commands

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/search"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// MaxCandidates is how many candidates an unknown id lists (SPEC §17.4).
const MaxCandidates = 3

// resolver turns the id arguments of show, set and promote into entries (SPEC §10, §17.4).
type resolver struct {
	entries []*store.Entry
	byID    map[string]*store.Entry
	byPath  map[string]*store.Entry
	byAlias map[string][]*store.Entry
}

func newResolver(entries []*store.Entry) *resolver {
	r := &resolver{entries: entries, byID: map[string]*store.Entry{},
		byPath: map[string]*store.Entry{}, byAlias: map[string][]*store.Entry{}}
	for _, e := range entries {
		if r.byID[e.ID] == nil {
			r.byID[e.ID] = e
		}
		r.byPath[e.Path] = e
		for _, a := range e.Fields.Strings("aliases") {
			if !slices.Contains(r.byAlias[a], e) {
				r.byAlias[a] = append(r.byAlias[a], e)
			}
		}
	}
	return r
}

// lookup resolves arg as an id, then as a store-relative path, then as an alias of exactly one
// entry. It returns nil when nothing matches, and the alias holders when several do.
func (r *resolver) lookup(arg string) (*store.Entry, []*store.Entry) {
	if e := r.byID[arg]; e != nil {
		return e, nil
	}
	if e := r.byPath[arg]; e != nil {
		return e, nil
	}
	switch hs := r.byAlias[arg]; len(hs) {
	case 0:
		return nil, nil
	case 1:
		return hs[0], nil
	default:
		return nil, hs
	}
}

// resolve returns the entry arg names. When there is none it prints the diagnostic of
// explain on standard error (SPEC §17.4) and returns nil.
func (r *resolver) resolve(e *Env, arg string) *store.Entry {
	en, _ := r.lookup(arg)
	if en == nil {
		fmt.Fprintf(e.Stderr, "memex: %s\n", r.explain(arg))
	}
	return en
}

// explain is the diagnostic for an arg that names no entry, without the "memex: " prefix:
// `no entry "<arg>"` and up to three candidates, by search score of arg, as continuation
// lines. An alias shared by several entries names none of them; its holders are the
// candidates.
func (r *resolver) explain(arg string) string {
	_, holders := r.lookup(arg)
	var b strings.Builder
	if holders != nil {
		fmt.Fprintf(&b, "no entry %q: it is an alias of %d entries; give an id", arg, len(holders))
	} else {
		fmt.Fprintf(&b, "no entry %q", arg)
		holders = r.candidates(arg)
	}
	for i, h := range holders {
		if i == MaxCandidates {
			break
		}
		fmt.Fprintf(&b, "\n  candidate: %s", h.Path)
	}
	return b.String()
}

var datePrefix = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}-`)

// idDistance is the Levenshtein distance from text to an id: the smaller of the distance to
// the whole id and to the id without its date prefix (SPEC §17.4).
func idDistance(text, id string) int {
	d := spec.Distance(text, id)
	if datePrefix.MatchString(id) {
		d = min(d, spec.Distance(text, id[len("YYYY-MM-DD-"):]))
	}
	return d
}

// candidates are the entries an unknown id may have meant (SPEC §17.4): every id within
// Levenshtein distance 3, nearest first and then by id, then the entries that rank highest
// for text as a search query, up to MaxCandidates in all.
func (r *resolver) candidates(text string) []*store.Entry {
	type near struct {
		e *store.Entry
		d int
	}
	var ns []near
	for _, e := range r.entries {
		if d := idDistance(text, e.ID); d <= spec.MaxSuggestDistance {
			ns = append(ns, near{e, d})
		}
	}
	slices.SortStableFunc(ns, func(a, b near) int {
		if a.d != b.d {
			return a.d - b.d
		}
		return strings.Compare(a.e.ID, b.e.ID)
	})
	var out []*store.Entry
	for _, n := range ns {
		out = append(out, n.e)
	}
	for _, h := range search.NewCorpus(r.entries).Search(search.ParseQuery(text).Items) {
		if !slices.Contains(out, h.Entry) {
			out = append(out, h.Entry)
		}
	}
	return out
}
