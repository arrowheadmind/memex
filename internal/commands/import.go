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
	Out     string // the fresh form's DIR; "" for the delta form
	Into    string // the delta form's existing store (SPEC §13.7)
	Leaks   []string
	Version string
}

// Import runs `memex import brain-v3` (SPEC §13). It needs no store.
func Import(e *Env, o ImportOptions) int {
	today, err := config.Today(e.Getenv, e.Now())
	if err != nil {
		return e.failErr(err)
	}
	if o.Into != "" {
		return importDelta(e, o, today)
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

// importDelta runs `memex import brain-v3 SRC… --into DIR` (SPEC §13.7).
func importDelta(e *Env, o ImportOptions, today time.Time) int {
	into, err := filepath.Abs(o.Into)
	if err != nil {
		return e.fail(spec.ExitUsage, "--into: %v", err)
	}
	if info, err := os.Stat(filepath.Join(into, "memex.toml")); err != nil || !info.Mode().IsRegular() {
		return e.fail(spec.ExitFailure, "%s is not a memex store; --into must name an existing store", o.Into)
	}
	report, d, name, err := importv3.Delta(importv3.Options{
		Sources: o.Sources, Leaks: o.Leaks, Today: today, Home: e.Getenv("HOME"),
		Version: o.Version, Timeout: 60 * time.Second, Stdout: e.Stdout, Stderr: e.Stderr,
	}, into)
	var refusal *importv3.Refusal
	if errors.As(err, &refusal) {
		return e.fail(refusal.Code, "%s", refusal.Msg)
	}
	var failure *importv3.Failure
	if errors.As(err, &failure) {
		printDelta(e, report, d, into, name)
		for _, p := range failure.Problems {
			fmt.Fprintf(e.Stderr, "memex: %s\n", p)
		}
		return e.fail(spec.ExitFailure, "import has %d problems; see %s", len(failure.Problems), filepath.Join(into, name))
	}
	if err != nil {
		return e.fail(spec.ExitFailure, "%v", err)
	}
	printDelta(e, report, d, into, name)
	return spec.ExitOK
}

func printDelta(e *Env, r *importv3.Report, d *importv3.DeltaReport, into, name string) {
	byType := map[string]int{}
	for _, a := range d.Added {
		byType[a.Type]++
	}
	fmt.Fprintf(e.Stdout, "imported %d new entries from %d sources into %s\n", len(d.Added), len(r.Sources), into)
	var types []string
	for _, t := range spec.Types {
		if byType[t.Name] > 0 {
			types = append(types, fmt.Sprintf("%s %d", t.Directory, byType[t.Name]))
		}
	}
	if len(types) > 0 {
		fmt.Fprintf(e.Stdout, "  new entries: %s\n", join(types))
	}
	fmt.Fprintf(e.Stdout, "  already in the store: %d unchanged, %d with a different body (left alone), %d under another id\n",
		d.Unchanged, len(d.Changed), len(d.Moved))
	fmt.Fprintf(e.Stdout, "  attachments added %d, different and left alone %d; aliases dropped %d\n",
		len(d.AttachmentsAdded), len(d.AttachmentsChanged), len(d.AliasesDropped))
	fmt.Fprintf(e.Stdout, "report: %s\n", filepath.Join(into, name))
}
