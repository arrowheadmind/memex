package commands

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// FmtOptions are the options of `memex fmt` (SPEC §11.9).
type FmtOptions struct {
	Paths []string
	Check bool
}

// storePath turns a path argument into a store-relative slash path: an absolute path must
// be inside the store; a relative one is taken as relative to the store root.
func storePath(root, p string) (string, bool) {
	if filepath.IsAbs(p) {
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", false
		}
		p = rel
	}
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." {
		return "", true
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// canonicalFiles returns the canonical form of every selected entry that is valid but not
// canonical (the entries check reports as W-CANONICAL). The others that are not canonical
// are reported on standard error, because fmt cannot rewrite them without changing a value.
// An unknown path is reported and makes ok false.
func (e *Env) canonicalFiles(root string, today time.Time, paths []string) (files []write.File, ok bool) {
	files, _, ok = e.nonCanonical(root, today, paths)
	return files, ok
}

// nonCanonical is canonicalFiles that also returns every selected path that is not in
// canonical form, the entries with errors and the unreadable ones included (`fmt --check`).
func (e *Env) nonCanonical(root string, today time.Time, paths []string) (files []write.File, all []string, ok bool) {
	tree := store.WorkTree{Root: root}
	entries, skipped, err := store.LoadEntries(tree)
	if err != nil {
		panic(err)
	}
	findings, err := check.Run(tree, today, nil)
	if err != nil {
		panic(err)
	}
	hasErr := map[string]bool{}
	for _, f := range findings {
		if check.IsError(f) {
			hasErr[f.Path] = true
		}
	}
	byPath := map[string]*store.Entry{}
	for _, en := range entries {
		byPath[en.Path] = en
	}
	unreadable := map[string]string{}
	for _, s := range skipped {
		unreadable[s.Path] = s.Problem
	}
	var selected []string
	if len(paths) == 0 {
		for _, en := range entries {
			selected = append(selected, en.Path)
		}
		for _, s := range skipped {
			selected = append(selected, s.Path)
		}
		slices.Sort(selected)
	}
	ok = true
	for _, p := range paths {
		rel, inside := storePath(root, p)
		if !inside || (byPath[rel] == nil && unreadable[rel] == "") {
			fmt.Fprintf(e.Stderr, "memex: no entry at %s\n", p)
			ok = false
			continue
		}
		if !slices.Contains(selected, rel) {
			selected = append(selected, rel)
		}
	}
	slices.Sort(selected)
	for _, p := range selected {
		if why, bad := unreadable[p]; bad {
			all = append(all, p)
			fmt.Fprintf(e.Stderr, "memex: skipped %s: %s; run memex check\n", p, why)
			continue
		}
		en := byPath[p]
		data := entry.Render(en.Fields, entry.CanonicalBody(en.Type, en.Body))
		if data == string(en.Data) {
			continue
		}
		all = append(all, p)
		if hasErr[p] {
			fmt.Fprintf(e.Stderr, "memex: skipped %s: it has errors; run memex check\n", p)
			continue
		}
		files = append(files, write.File{Path: p, Data: []byte(data), Check: true, Expect: en.Data})
	}
	return files, all, ok
}

// Fmt runs `memex fmt` (SPEC §11.9).
func Fmt(e *Env, o FmtOptions) int {
	if o.Check {
		_, today, root, err := e.openStore()
		if err != nil {
			return e.failErr(err)
		}
		if code, ok := e.checkManifest(root); !ok {
			return code
		}
		_, all, ok := e.nonCanonical(root, today, o.Paths)
		for _, p := range all {
			fmt.Fprintln(e.Stdout, p)
		}
		if !ok || len(all) > 0 {
			return spec.ExitFailure
		}
		return spec.ExitOK
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code
	}
	files, ok := e.canonicalFiles(w.root, w.today, o.Paths)
	if !ok {
		w.end(nil)
		return spec.ExitFailure
	}
	if len(files) == 0 {
		return w.nothing()
	}
	return w.commit(files, fmt.Sprintf("memex: fmt %d entries", len(files)))
}
