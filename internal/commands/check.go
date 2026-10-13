package commands

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// CheckOptions are the options of `memex check` (SPEC §11.12).
type CheckOptions struct {
	Staged, Strict, ErrorsOnly, JSON bool
	Limit                            int
}

// DefaultCheckLimit is the default --limit of check (SPEC §7).
const DefaultCheckLimit = 200

// Check runs `memex check` (SPEC §7, §11.12).
func Check(e *Env, o CheckOptions) int {
	_, today, root, err := e.openStore()
	if err != nil {
		return e.failErr(err)
	}
	var tree store.Tree = store.WorkTree{Root: root}
	var staged []string
	if o.Staged {
		g := &gitx.Git{Dir: root, Env: gitx.Env(e.Environ, true)}
		idx, changed, err := store.ReadIndex(g)
		if err != nil {
			return e.fail(spec.ExitUsage, "check --staged needs the store's git index: %v", err)
		}
		if !idx.Exists(store.ManifestFile) {
			return e.fail(spec.ExitUsage, "check --staged: %s is not in the git index", store.ManifestFile)
		}
		tree, staged = idx, changed
		if staged == nil {
			staged = []string{}
		}
	}
	findings, err := check.Run(tree, today, staged)
	var fe *store.FormatError
	if errors.As(err, &fe) {
		return e.fail(spec.ExitUsage, "%s", fe.Msg)
	}
	if err != nil {
		panic(err)
	}

	errs, warns := 0, 0
	for _, f := range findings {
		if check.IsError(f) {
			errs++
		} else {
			warns++
		}
	}
	shown := findings
	if o.ErrorsOnly {
		shown = slices.DeleteFunc(slices.Clone(findings), func(f check.Finding) bool { return !check.IsError(f) })
	}
	listed, rest := shown, []check.Finding(nil)
	if len(shown) > o.Limit {
		listed, rest = shown[:o.Limit], shown[o.Limit:]
	}

	if o.JSON {
		items := make(jsonx.Array, len(listed))
		for i, f := range listed {
			sev := "warning"
			if check.IsError(f) {
				sev = "error"
			}
			items[i] = jsonx.Object{
				{Key: "path", Value: jsonx.String(f.Path)},
				{Key: "code", Value: jsonx.String(f.Code)},
				{Key: "severity", Value: jsonx.String(sev)},
				{Key: "message", Value: jsonx.String(f.Message)},
			}
		}
		e.Stdout.Write(jsonx.Encode(jsonx.Object{
			{Key: "findings", Value: items},
			{Key: "errors", Value: jsonx.Int(errs)},
			{Key: "warnings", Value: jsonx.Int(warns)},
			{Key: "truncated", Value: jsonx.Int(len(rest))},
		}))
	} else {
		var b strings.Builder
		for _, f := range listed {
			fmt.Fprintf(&b, "%s: %s %s\n", f.Path, f.Code, f.Message)
		}
		more := map[string]int{}
		var codes []string
		for _, f := range rest {
			if more[f.Code] == 0 {
				codes = append(codes, f.Code)
			}
			more[f.Code]++
		}
		slices.Sort(codes)
		for _, c := range codes {
			fmt.Fprintf(&b, "%s: %d more\n", c, more[c])
		}
		fmt.Fprintf(&b, "check: %d errors, %d warnings\n", errs, warns)
		io.WriteString(e.Stdout, b.String())
	}
	if errs > 0 || (o.Strict && warns > 0) {
		return spec.ExitFailure
	}
	return spec.ExitOK
}
