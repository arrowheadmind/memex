package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/spec"
)

// SaveOptions are the options of `memex save` (SPEC §9.3).
type SaveOptions struct {
	Message     string
	MessageSet  bool
	Paths       []string
	AllowDelete bool
	Fmt         bool
}

// change is one path of the working tree that differs from HEAD.
type change struct {
	path    string
	deleted bool
}

// changes lists every added, modified, deleted or untracked file of the store, except
// .memex/ and what .gitignore excludes, as store-relative paths in code-point order.
func changes(root string) ([]change, error) {
	g := gitx.New(root, 0)
	prefix, err := g.Run("rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	prefix = strings.TrimSuffix(prefix, "\n")
	out, err := g.Run("status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames", "--", ".")
	if err != nil {
		return nil, err
	}
	var cs []change
	for _, rec := range strings.Split(out, "\x00") {
		if len(rec) < 4 {
			continue
		}
		xy, p := rec[:2], rec[3:]
		p = strings.TrimPrefix(p, prefix)
		if p == ".memex" || strings.HasPrefix(p, ".memex/") {
			continue
		}
		cs = append(cs, change{path: p, deleted: xy[0] == 'D' || xy[1] == 'D'})
	}
	slices.SortFunc(cs, func(a, b change) int { return strings.Compare(a.path, b.path) })
	return cs, nil
}

// isEntryPath reports whether p is where an entry lives: a .md file directly in a type
// directory.
func isEntryPath(p string) bool {
	dir, name, ok := strings.Cut(p, "/")
	return ok && spec.LookupDirectory(dir) != nil && !strings.Contains(name, "/") && strings.HasSuffix(name, ".md")
}

// Save runs `memex save` (SPEC §9.3): it commits direct edits through §9.1 steps 1–2 and
// 5–8, with step 4 only for --fmt.
func Save(e *Env, o SaveOptions) (int, *UsageError) {
	if o.MessageSet && strings.TrimSpace(o.Message) == "" {
		return 0, &UsageError{"empty value for -m; give a commit message, or leave -m out"}
	}
	w, code, ok := e.beginWriteFor(true, false)
	if !ok {
		return code, nil
	}
	all, err := changes(w.root)
	if err != nil {
		panic(err)
	}
	selected := all
	if len(o.Paths) > 0 {
		selected = nil
		for _, arg := range o.Paths {
			p, inside := storePath(w.root, arg)
			if !inside {
				return w.refuse(spec.ExitFailure, "%s is outside the store", arg), nil
			}
			for _, c := range all {
				if (p == "" || c.path == p || strings.HasPrefix(c.path, p+"/")) && !slices.Contains(selected, c) {
					selected = append(selected, c)
				}
			}
		}
		slices.SortFunc(selected, func(a, b change) int { return strings.Compare(a.path, b.path) })
	}
	if len(selected) == 0 {
		return w.nothing(), nil
	}
	var paths, present []string
	for _, c := range selected {
		paths = append(paths, c.path)
		if !c.deleted {
			present = append(present, c.path)
			continue
		}
		if isEntryPath(c.path) && !o.AllowDelete {
			id := strings.TrimSuffix(c.path[strings.Index(c.path, "/")+1:], ".md")
			return w.refuse(spec.ExitFailure, "%s is a deleted entry; ids are never reused, so retire or supersede it instead "+
				"(memex set %s status=…), restore it with git restore -- %s, or give --allow-delete", c.path, id, c.path), nil
		}
	}
	if o.Fmt {
		var entries []string
		for _, p := range present {
			if isEntryPath(p) {
				entries = append(entries, p)
			}
		}
		if len(entries) > 0 {
			files, _ := e.canonicalFiles(w.root, w.today, entries)
			if err := w.s.Write(w.today, files); err != nil {
				return w.failed(err), nil
			}
		}
	}
	if err := w.s.Check(w.today, present); err != nil {
		w.s.Restore()
		return w.failed(err), nil
	}
	msg := fmt.Sprintf("memex: save %d files", len(paths))
	if o.MessageSet {
		msg = o.Message
	}
	return w.commitPaths(paths, msg), nil
}
