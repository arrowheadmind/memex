package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/search"
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

// resolve returns the entry arg names. When there is none it prints `no entry "<arg>"` and up
// to three candidates, by search score of arg, on standard error (SPEC §17.4) and returns
// nil. An alias shared by several entries names none of them; its holders are the candidates.
func (r *resolver) resolve(e *Env, arg string) *store.Entry {
	en, holders := r.lookup(arg)
	if en != nil {
		return en
	}
	var b strings.Builder
	if holders != nil {
		fmt.Fprintf(&b, "memex: no entry %q: it is an alias of %d entries; give an id\n", arg, len(holders))
	} else {
		fmt.Fprintf(&b, "memex: no entry %q\n", arg)
		for _, it := range search.NewCorpus(r.entries).Search(search.ParseQuery(arg).Items) {
			holders = append(holders, it.Entry)
		}
	}
	for i, h := range holders {
		if i == MaxCandidates {
			break
		}
		fmt.Fprintf(&b, "  candidate: %s\n", h.Path)
	}
	fmt.Fprint(e.Stderr, b.String())
	return nil
}
