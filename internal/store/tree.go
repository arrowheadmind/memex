package store

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/arrowheadmind/memex/internal/gitx"
)

// Item is one name inside a directory of a Tree.
type Item struct {
	Name    string
	Dir     bool
	Regular bool // a regular file (not a symbolic link, submodule or special file)
}

// Tree is a read-only view of the store's files: the working tree, or the snapshot in the git
// index that `check --staged` validates. Paths are slash-separated and relative to the root.
type Tree interface {
	// Children lists the items of directory dir ("" is the root), sorted by name.
	Children(dir string) ([]Item, error)
	// ReadFile returns the bytes of a file.
	ReadFile(p string) ([]byte, error)
	// Locate resolves p, following symbolic links, and says whether it names a regular file
	// or a directory whose real path is inside the store (SPEC §6, E-FILE-LINK).
	Locate(p string) Location
}

// Location is the result of Tree.Locate.
type Location int

const (
	Inside  Location = iota // a regular file or directory inside the store
	Missing                 // nothing there, a dangling link, or something else
	Outside                 // resolves to a path outside the store
)

// WorkTree is the store's working tree on disk.
type WorkTree struct{ Root string }

// Children implements Tree.
func (w WorkTree) Children(dir string) ([]Item, error) {
	des, err := os.ReadDir(filepath.Join(w.Root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, err
	}
	items := make([]Item, len(des))
	for i, de := range des {
		items[i] = Item{Name: de.Name(), Dir: de.IsDir(), Regular: de.Type().IsRegular()}
	}
	return items, nil
}

// ReadFile implements Tree.
func (w WorkTree) ReadFile(p string) ([]byte, error) {
	return os.ReadFile(filepath.Join(w.Root, filepath.FromSlash(p)))
}

// Locate implements Tree.
func (w WorkTree) Locate(p string) Location {
	real, err := filepath.EvalSymlinks(filepath.Join(w.Root, filepath.FromSlash(p)))
	if err != nil {
		return Missing
	}
	root, err := filepath.EvalSymlinks(w.Root)
	if err != nil {
		return Missing
	}
	if rel, err := filepath.Rel(root, real); err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return Outside
	}
	info, err := os.Stat(real)
	if err != nil || !(info.Mode().IsRegular() || info.IsDir()) {
		return Missing
	}
	return Inside
}

// IndexTree is the snapshot of the store held in the git index (GIT_INDEX_FILE when set).
type IndexTree struct {
	root  string // the store root, to recognise absolute links into the store
	files map[string]indexFile
	dirs  map[string][]Item
}

type indexFile struct {
	mode, object string
	data         []byte
}

// ReadIndex loads every path of the index with its contents, and returns the paths that are
// added or modified relative to HEAD (every path when there is no HEAD).
func ReadIndex(root string, g *gitx.Git) (*IndexTree, []string, error) {
	out, err := g.Run("ls-files", "--stage", "-z")
	if err != nil {
		return nil, nil, err
	}
	t := &IndexTree{root: root, files: map[string]indexFile{}, dirs: map[string][]Item{}}
	var objects []string
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		meta, p, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, nil, fmt.Errorf("git ls-files: unexpected record %q", rec)
		}
		if _, seen := t.files[p]; seen { // an unmerged path has several stages
			continue
		}
		t.files[p] = indexFile{mode: fields[0], object: fields[1]}
		if fields[0] != "160000" {
			objects = append(objects, fields[1])
		}
	}
	blobs, err := catBlobs(g, objects)
	if err != nil {
		return nil, nil, err
	}
	seenDir := map[string]map[string]bool{}
	add := func(dir string, it Item) {
		if seenDir[dir] == nil {
			seenDir[dir] = map[string]bool{}
		}
		if !seenDir[dir][it.Name] {
			seenDir[dir][it.Name] = true
			t.dirs[dir] = append(t.dirs[dir], it)
		}
	}
	for p, f := range t.files {
		f.data = blobs[f.object]
		t.files[p] = f
		dir, name := path.Split(p)
		dir = strings.TrimSuffix(dir, "/")
		add(dir, Item{Name: name, Regular: f.mode == "100644" || f.mode == "100755"})
		for dir != "" {
			parent, name := path.Split(dir)
			parent = strings.TrimSuffix(parent, "/")
			add(parent, Item{Name: name, Dir: true})
			dir = parent
		}
	}
	for d := range t.dirs {
		slices.SortFunc(t.dirs[d], func(a, b Item) int { return strings.Compare(a.Name, b.Name) })
	}

	var changed []string
	if _, err := g.Run("rev-parse", "--verify", "-q", "HEAD"); err != nil {
		for p := range t.files {
			changed = append(changed, p)
		}
	} else {
		out, err := g.Run("diff", "--cached", "--name-only", "--no-renames", "--diff-filter=AMT", "-z", "HEAD")
		if err != nil {
			return nil, nil, err
		}
		for _, p := range strings.Split(out, "\x00") {
			if p != "" {
				changed = append(changed, p)
			}
		}
	}
	slices.Sort(changed)
	return t, changed, nil
}

// catBlobs reads the given objects with one `git cat-file --batch`.
func catBlobs(g *gitx.Git, objects []string) (map[string][]byte, error) {
	blobs := map[string][]byte{}
	if len(objects) == 0 {
		return blobs, nil
	}
	out, err := g.RunInput([]byte(strings.Join(objects, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(strings.NewReader(out))
	for range objects {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: %v", err)
		}
		f := strings.Fields(header)
		if len(f) == 2 && f[1] == "missing" {
			return nil, fmt.Errorf("object %s is missing", f[0])
		}
		if len(f) != 3 {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", header)
		}
		size, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", header)
		}
		data := make([]byte, size+1)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("git cat-file --batch: %v", err)
		}
		blobs[f[0]] = data[:size]
	}
	return blobs, nil
}

// Children implements Tree.
func (t *IndexTree) Children(dir string) ([]Item, error) {
	items, ok := t.dirs[dir]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrNotExist}
	}
	return items, nil
}

// ReadFile implements Tree.
func (t *IndexTree) ReadFile(p string) ([]byte, error) {
	f, ok := t.files[p]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: p, Err: fs.ErrNotExist}
	}
	return f.data, nil
}

// maxLinkHops bounds symbolic-link resolution, as the kernel's ELOOP does.
const maxLinkHops = 40

// Locate implements Tree. Symbolic links in the index are resolved against the index.
func (t *IndexTree) Locate(p string) Location {
	parts := strings.Split(p, "/")
	var resolved []string
	for hops := 0; len(parts) > 0; {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return Outside
			}
			resolved = resolved[:len(resolved)-1]
			continue
		}
		cur := path.Join(append(slices.Clone(resolved), part)...)
		f, ok := t.files[cur]
		if !ok || f.mode != "120000" {
			resolved = append(resolved, part)
			continue
		}
		if hops++; hops > maxLinkHops {
			return Missing
		}
		target := string(f.data)
		if path.IsAbs(target) {
			rel, ok := strings.CutPrefix(target, t.root+"/")
			if !ok {
				return Outside
			}
			resolved, target = nil, rel
		}
		parts = append(strings.Split(target, "/"), parts...)
	}
	final := path.Join(resolved...)
	if final == "" {
		return Inside
	}
	if f, ok := t.files[final]; ok && (f.mode == "100644" || f.mode == "100755") {
		return Inside
	}
	if _, ok := t.dirs[final]; ok {
		return Inside
	}
	return Missing
}

// Exists reports whether a file or directory is at p in the index.
func (t *IndexTree) Exists(p string) bool {
	_, isFile := t.files[p]
	_, isDir := t.dirs[p]
	return isFile || isDir
}

// IsNotExist reports whether err says a path does not exist.
func IsNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
