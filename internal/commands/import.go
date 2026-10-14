package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/importv3"
	"github.com/arrowheadmind/memex/internal/spec"
)

// ImportOptions are the options of `memex import brain-v3` (SPEC §13).
type ImportOptions struct {
	Sources []string
	Out     string
	Leaks   []string
	Version string
}

// Import runs `memex import brain-v3` (SPEC §13). It needs no store.
func Import(e *Env, o ImportOptions) int {
	today, err := config.Today(e.Getenv, e.Now())
	if err != nil {
		return e.failErr(err)
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return e.fail(spec.ExitUsage, "--out: %v", err)
	}
	if _, err := os.Lstat(out); err == nil {
		return e.fail(spec.ExitFailure, "%s exists; --out must name a new directory", o.Out)
	}
	report, err := importv3.Run(importv3.Options{
		Sources: o.Sources, Out: out, Leaks: o.Leaks, Today: today, Home: e.Getenv("HOME"),
		Version: o.Version, Timeout: 60 * time.Second, Stdout: e.Stdout, Stderr: e.Stderr,
	})
	var failure *importv3.Failure
	if errors.As(err, &failure) {
		printSummary(e, report, out)
		for _, p := range failure.Problems {
			fmt.Fprintf(e.Stderr, "memex: %s\n", p)
		}
		return e.fail(spec.ExitFailure, "import has %d problems; see %s", len(failure.Problems), filepath.Join(out, "import-report.json"))
	}
	if err != nil {
		return e.fail(spec.ExitFailure, "%v", err)
	}
	printSummary(e, report, out)
	return spec.ExitOK
}

func printSummary(e *Env, r *importv3.Report, out string) {
	byType := map[string]int{}
	for _, en := range r.Entries {
		byType[en.Type]++
	}
	fmt.Fprintf(e.Stdout, "imported %d entries from %d sources into %s\n", len(r.Entries), len(r.Sources), out)
	var types []string
	for _, t := range spec.Types {
		if byType[t.Name] > 0 {
			types = append(types, fmt.Sprintf("%s %d", t.Directory, byType[t.Name]))
		}
	}
	fmt.Fprintf(e.Stdout, "  entries: %s\n", join(types))
	fmt.Fprintf(e.Stdout, "  attachments %d, not-knowledge %d files, leaks %d\n", len(r.Attachments), len(r.NotKnowledge), len(r.Leaks))
	fmt.Fprintf(e.Stdout, "  index rows %d: matched %d, orphan %d, duplicate %d, conflict %d, placeholders %d\n",
		r.Index.Rows, r.Index.Matched, len(r.Index.Orphan), len(r.Index.Duplicate), len(r.Index.Conflict), r.Index.Placeholders)
	fmt.Fprintf(e.Stdout, "  links rewritten %d, unresolved targets %d; duplicates dropped %d; short bodies %d\n",
		len(r.Links.Rewritten), len(r.Links.Unresolved), len(r.Duplicates), len(r.ShortBody))
	var warns []string
	for _, w := range r.Warnings {
		warns = append(warns, fmt.Sprintf("%s %d", w.Target, w.Count))
	}
	sort.Strings(warns)
	if len(warns) > 0 {
		fmt.Fprintf(e.Stdout, "  warnings: %s\n", join(warns))
	}
	fmt.Fprintf(e.Stdout, "report: %s\n", filepath.Join(out, "import-report.json"))
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
