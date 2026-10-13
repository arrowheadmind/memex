package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/search"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// Preflight budget bounds (SPEC §11.4).
const (
	DefaultMaxChars = 12000
	MinMaxChars     = 2000
)

// PreflightOptions are the options of `memex preflight` (SPEC §11.4).
type PreflightOptions struct {
	Query    *search.Query // nil without a query
	NoSync   bool
	MaxChars int
	Filter   search.Filter
	Limit    int // -1 means search_limit; 0 means no limit
}

// lastFetch is the modification time of the store's FETCH_HEAD, which every git fetch
// rewrites, in RFC 3339 UTC, or "never". The store root must be the top of its own
// repository: a store directory inside another repository has never been fetched itself.
func lastFetch(root string) string {
	g := gitx.New(root, 0)
	top, err := g.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return "never"
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Clean(strings.TrimSuffix(top, "\n")) != filepath.Clean(real) {
		return "never"
	}
	p, err := g.Run("rev-parse", "--git-path", "FETCH_HEAD")
	if err != nil {
		return "never"
	}
	p = strings.TrimSuffix(p, "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	st, err := os.Stat(p)
	if err != nil {
		return "never"
	}
	return st.ModTime().UTC().Truncate(time.Second).Format(time.RFC3339)
}

func codePoints(s string) int { return utf8.RuneCountInString(s) }

// Preflight runs `memex preflight` (SPEC §11.4). The sync of §9.5 is not implemented yet:
// unless sync is skipped, it is a usage error.
func Preflight(e *Env, o PreflightOptions) (int, *UsageError) {
	cfg, err := config.Load(e.Getenv)
	if err != nil {
		return e.failErr(err), nil
	}
	if !o.NoSync && cfg.SyncOnPreflight {
		return 0, &UsageError{"sync is not implemented yet; give --no-sync or set sync_on_preflight = false"}
	}
	rs, code, ok := e.loadEntries()
	if !ok {
		return code, nil
	}
	var head strings.Builder
	fmt.Fprintf(&head, "store: not synced (last fetch %s)\n\n## Always\n", lastFetch(rs.root))
	var rules []*store.Entry
	for _, en := range rs.entries {
		if en.Type.Name == "rule" && en.Visible() && en.Fields["always"] == true {
			rules = append(rules, en)
		}
	}
	slices.SortFunc(rules, func(a, b *store.Entry) int { return strings.Compare(a.ID, b.ID) })
	always := len(rules)
	for _, en := range rules {
		head.WriteString("- " + en.Path + ": " + en.Fields.String("title"))
		if s := en.Fields.String("summary"); s != "" {
			head.WriteString(" — " + s)
		}
		head.WriteString("\n")
	}
	var warning string
	if n := codePoints(head.String()); 2*n > o.MaxChars {
		warning = fmt.Sprintf("warning: always rules use %d of %d characters; review them\n", n, o.MaxChars)
	}
	if o.Query == nil {
		fmt.Fprint(e.Stdout, head.String()+warning)
		return spec.ExitOK, nil
	}

	limit := o.Limit
	if limit < 0 {
		limit = rs.cfg.SearchLimit
	}
	res := search.NewCorpus(rs.entries).Run(*o.Query, &o.Filter, limit)
	blocks := make([]string, len(res.Shown))
	for i := range res.Shown {
		blocks[i] = res.Block(i)
	}
	// Results are dropped from the end, whole blocks at a time, until the output fits.
	render := func(k int) string {
		var b strings.Builder
		b.WriteString(head.String() + "\n## Results\n")
		if !res.Matched() {
			b.WriteString(res.NoMatch())
		} else {
			b.WriteString(strings.Join(blocks[:k], ""))
			b.WriteString(res.Trailer(rs.root, k))
		}
		b.WriteString(warning)
		if k < len(blocks) {
			fmt.Fprintf(&b, "output budget reached: %d results not shown (raise --max-chars or run memex search)\n", len(blocks)-k)
		}
		return b.String()
	}
	k := len(blocks)
	out := render(k)
	for k > 0 && codePoints(out) > o.MaxChars {
		k--
		out = render(k)
	}
	fmt.Fprint(e.Stdout, out)
	code = res.ExitCode()
	if code == spec.ExitNoMatch && always > 0 {
		code = spec.ExitOK
	}
	return code, nil
}
