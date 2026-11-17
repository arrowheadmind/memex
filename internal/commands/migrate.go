package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// MigrateOptions are the options of `memex migrate` (SPEC §11.19).
type MigrateOptions struct {
	To int64
}

var (
	formatLine    = regexp.MustCompile(`^[ \t]*format[ \t]*=`)
	staleDaysLine = regexp.MustCompile(`(?m)^[ \t]*stale_days[ \t]*=`)
)

// Migrate runs `memex migrate` (SPEC §11.19): it rewrites memex.toml to another store format
// in one commit, after checking that every entry is valid at that format.
func Migrate(e *Env, o MigrateOptions) int {
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code
	}
	// Only memex.toml is committed, so the store must be clean: the validation below reads the
	// working tree, and an entry fixed but not saved would leave HEAD invalid at the new format.
	if cs, err := changes(w.root); err != nil {
		panic(err)
	} else if len(cs) > 0 {
		what := fmt.Sprintf("%d uncommitted changes", len(cs))
		if len(cs) == 1 {
			what = "1 uncommitted change"
		}
		return w.refuse(spec.ExitFailure, "the store has %s; run memex save first", what)
	}
	data, err := os.ReadFile(filepath.Join(w.root, store.ManifestFile))
	if err != nil {
		panic(err)
	}
	m, mf, err := store.ParseManifest(data)
	if err != nil {
		panic(err) // beginWrite refused a format this memex cannot read
	}
	if o.To >= spec.DueFormat {
		// Migrating is what resolves "stale_days needs store format 2" (SPEC §11.19 step 2).
		mf = slices.DeleteFunc(mf, func(f store.Finding) bool { return f.Message == store.StaleDaysNeedsFormat })
	}
	if len(mf) > 0 {
		for _, f := range mf {
			fmt.Fprintf(e.Stderr, "memex: %s: %s %s\n", f.Path, f.Code, f.Message)
		}
		return w.refuse(spec.ExitFailure, "refused: memex.toml is not valid; fix it first (memex check lists the findings)")
	}
	if m.Format == o.To {
		return w.nothing(fmt.Sprintf("store format %d already; nothing to migrate", o.To))
	}
	if o.To < spec.DueFormat {
		if code, refused := w.refuseDueFields(); refused {
			return code
		}
	}
	next := migratedManifest(data, o.To)
	added, err := addedErrors(w, next)
	if err != nil {
		panic(err)
	}
	if len(added) > 0 {
		for _, f := range added {
			fmt.Fprintf(e.Stderr, "memex: %s: %s %s\n", f.Path, f.Code, f.Message)
		}
		return w.refuse(spec.ExitFailure, "refused: the store would not validate at format %d; fix these entries first, nothing was changed", o.To)
	}
	lines := []string{fmt.Sprintf("store format %d -> %d", m.Format, o.To)}
	if o.To >= spec.DueFormat && !staleDaysLine.Match(data) {
		lines = append(lines, fmt.Sprintf("stale_days = %d written: check now reports open tasks unchanged for more than %d days (W-OPEN-STALE); stale_days = 0 turns it off",
			store.DefaultStaleDays, store.DefaultStaleDays))
	}
	files := []write.File{{Path: store.ManifestFile, Data: next, Check: true, Expect: data}}
	return w.commit(files, fmt.Sprintf("memex: migrate the store to format %d", o.To), lines...)
}

// refuseDueFields refuses a move to a format before spec.DueFormat while an entry carries due
// or due_when, naming each one and the command that removes the fields.
func (w *writing) refuseDueFields() (int, bool) {
	var carriers []*store.Entry
	for _, en := range w.entries() {
		if slices.ContainsFunc(spec.DueFields, en.Fields.Has) {
			carriers = append(carriers, en)
		}
	}
	if len(carriers) == 0 {
		return 0, false
	}
	for _, en := range carriers {
		var names, assigns []string
		for _, f := range spec.DueFields {
			if en.Fields.Has(f) {
				names = append(names, f)
				assigns = append(assigns, f+"=")
			}
		}
		fmt.Fprintf(w.e.Stderr, "memex: %s: carries %s; remove it first: memex set %s %s\n",
			en.Path, strings.Join(names, " and "), en.ID, strings.Join(assigns, " "))
	}
	what := fmt.Sprintf("%d entries carry", len(carriers))
	if len(carriers) == 1 {
		what = "1 entry carries"
	}
	return w.refuse(spec.ExitFailure, "refused: %s due or due_when, which store format %d does not allow; nothing was changed",
		what, spec.DueFormat-1), true
}

// migratedManifest returns data with its format line set to `format = to`, and stale_days
// added with its default when moving to a format that has it and absent, or removed when
// moving to one that does not. Every other line is kept as it is.
func migratedManifest(data []byte, to int64) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	var out []string
	hasStale := false
	for _, l := range lines {
		switch {
		case formatLine.MatchString(l):
			out = append(out, fmt.Sprintf("format = %d\n", to))
		case staleDaysLine.MatchString(l):
			if to >= spec.DueFormat {
				hasStale = true
				out = append(out, l)
			}
		default:
			out = append(out, l)
		}
	}
	s := strings.Join(out, "")
	if to >= spec.DueFormat && !hasStale {
		if s != "" && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		s += fmt.Sprintf("stale_days = %d\n", store.DefaultStaleDays)
	}
	return []byte(s)
}

// addedErrors returns the errors the store would have with memex.toml replaced by manifest
// that it does not have now.
func addedErrors(w *writing, manifest []byte) ([]store.Finding, error) {
	base := store.WorkTree{Root: w.root}
	before, err := check.Run(base, w.today, nil)
	if err != nil {
		return nil, err
	}
	after, err := check.Run(&write.Pending{Base: base, Files: map[string][]byte{store.ManifestFile: manifest}}, w.today, nil)
	if err != nil {
		return nil, err
	}
	had := map[store.Finding]bool{}
	for _, f := range before {
		had[f] = true
	}
	var out []store.Finding
	for _, f := range after {
		if check.IsError(f) && !had[f] {
			out = append(out, f)
		}
	}
	return out, nil
}
