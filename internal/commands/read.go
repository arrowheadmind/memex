package commands

import (
	"errors"
	"fmt"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/search"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// UsageError is a usage error a command finds only after loading the configuration. main
// prints it with the command's usage line (SPEC §16.8).
type UsageError struct{ Problem string }

// readStore is what the read commands work on.
type readStore struct {
	cfg     *config.Config
	root    string
	entries []*store.Entry
}

// loadEntries opens the store and reads its entries. An entry that cannot be read is
// reported on standard error, so that its absence from the results is not silent. On failure
// it returns the exit code to use.
func (e *Env) loadEntries() (*readStore, int, bool) {
	cfg, _, root, err := e.openStore()
	if err != nil {
		return nil, e.failErr(err), false
	}
	entries, skipped, err := store.LoadEntries(store.WorkTree{Root: root})
	var fe *store.FormatError
	if errors.As(err, &fe) {
		return nil, e.fail(spec.ExitUsage, "%s", fe.Msg), false
	}
	if err != nil {
		panic(err)
	}
	for _, s := range skipped {
		fmt.Fprintf(e.Stderr, "memex: skipped %s: %s; run memex check\n", s.Path, s.Problem)
	}
	return &readStore{cfg, root, entries}, 0, true
}

// SearchOptions are the options of `memex search` (SPEC §11.3).
type SearchOptions struct {
	Query  search.Query
	Filter search.Filter
	Limit  int // -1 means search_limit; 0 means no limit
	JSON   bool
}

// Search runs `memex search` (SPEC §8, §11.3).
func Search(e *Env, o SearchOptions) int {
	rs, code, ok := e.loadEntries()
	if !ok {
		return code
	}
	limit := o.Limit
	if limit < 0 {
		limit = rs.cfg.SearchLimit
	}
	res := search.NewCorpus(rs.entries).Run(o.Query, &o.Filter, limit)
	if o.JSON {
		e.Stdout.Write(res.JSON())
	} else {
		fmt.Fprint(e.Stdout, res.TextOutput(rs.root))
	}
	return res.ExitCode()
}
