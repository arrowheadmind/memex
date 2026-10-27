package commands

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/neutral"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// CheckOptions are the options of `memex check` (SPEC §11.12).
type CheckOptions struct {
	Staged, Strict, ErrorsOnly, Immutable, JSON bool
	Workspace                                   bool // add the §12 pointer and leak findings
	Limit                                       int  // 0 means no limit
}

// DefaultCheckLimit is the default --limit of check (SPEC §7).
const DefaultCheckLimit = 200

// runHistoryCheck runs the history-dependent checks and, only when the first pass finds
// immutable candidates, supplies their HEAD bodies for a final classification pass.
func runHistoryCheck(tree store.Tree, today time.Time, staged []string, hist *store.History, g *gitx.Git) ([]check.Finding, error, error) {
	findings, err := check.RunHistory(tree, today, staged, hist)
	if err != nil || !hist.HasHEAD {
		return findings, err, nil
	}
	wanted := check.ImmutableCandidates(findings)
	if len(wanted) == 0 {
		return findings, nil, nil
	}
	heads, historyErr := store.ReadHeadFiles(g, spec.LookupType("journal").Directory, wanted)
	if historyErr != nil {
		return nil, nil, historyErr
	}
	for p, data := range heads {
		hist.Versions[p] = [][]byte{data}
	}
	findings, err = check.RunHistory(tree, today, staged, hist)
	return findings, err, nil
}

// Check runs `memex check` (SPEC §7, §11.12).
func Check(e *Env, o CheckOptions) int {
	cfg, today, root, err := e.openStore()
	if err != nil {
		return e.failErr(err)
	}
	var scan *neutral.Scan
	if o.Workspace {
		ws, err := config.ResolveWorkspace(e.Getenv, cfg, root)
		if err != nil {
			return e.failErr(err)
		}
		scan = &neutral.Scan{Home: e.Getenv("HOME"), Workspace: ws, Store: root}
	}
	var tree store.Tree = store.WorkTree{Root: root}
	var staged []string
	g := gitx.New(root, 0)
	if o.Staged {
		g = gitx.New(root, 0, gitx.Staged())
		idx, changed, err := store.ReadIndex(root, g)
		if err != nil {
			return e.fail(spec.ExitUsage, "check --staged needs the store's git index: %v", err)
		}
		if len(changed) > 0 && !idx.Exists(store.ManifestFile) {
			return e.fail(spec.ExitUsage, "check --staged: %s is not in the git index", store.ManifestFile)
		}
		tree, staged = idx, changed
	}
	var findings []check.Finding
	if !o.Staged || len(staged) > 0 { // nothing staged, an empty index included: nothing to check
		hist, herr := store.ReadHistory(g, check.CurrencyDirs())
		if herr != nil {
			return e.fail(spec.ExitUsage, "check needs the store's git history: %s", store.HistoryReason(herr, 0))
		}
		findings, err, herr = runHistoryCheck(tree, today, staged, hist, g)
		if herr != nil {
			return e.fail(spec.ExitUsage, "check needs the store's git history: %s", store.HistoryReason(herr, 0))
		}
	}
	var fe *store.FormatError
	if errors.As(err, &fe) {
		return e.fail(spec.ExitUsage, "%s", fe.Msg)
	}
	if err != nil {
		panic(err)
	}
	if scan != nil {
		findings = append(findings, scan.Pointers()...)
		findings = append(findings, neutral.LeakFindings(scan.Leaks())...)
		check.Sort(findings)
	}

	errs, warns, immutable := 0, 0, 0
	for _, f := range findings {
		if f.Immutable {
			immutable++
		} else if check.IsError(f) {
			errs++
		} else {
			warns++
		}
	}
	shown := findings
	shown = slices.DeleteFunc(slices.Clone(shown), func(f check.Finding) bool {
		return (f.Immutable && !o.Immutable) || (o.ErrorsOnly && !check.IsError(f) && !f.Immutable)
	})
	listed, rest := shown, []check.Finding(nil)
	if o.Limit > 0 && len(shown) > o.Limit {
		listed, rest = shown[:o.Limit], shown[o.Limit:]
	}

	if o.JSON {
		items := make(jsonx.Array, len(listed))
		for i, f := range listed {
			sev := "warning"
			if f.Immutable {
				sev = "immutable"
			} else if check.IsError(f) {
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
			{Key: "immutable", Value: jsonx.Int(immutable)},
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
			fmt.Fprintf(&b, "%s: %d more (raise --limit)\n", c, more[c])
		}
		fmt.Fprintf(&b, "check: %d errors, %d warnings", errs, warns)
		if immutable > 0 {
			fmt.Fprintf(&b, ", %d on immutable journal bodies", immutable)
			if !o.Immutable {
				b.WriteString(" (memex check --immutable lists them)")
			}
		}
		b.WriteByte('\n')
		io.WriteString(e.Stdout, b.String())
	}
	if errs > 0 || (o.Strict && warns > 0) {
		return spec.ExitFailure
	}
	return spec.ExitOK
}
