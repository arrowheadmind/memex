package write

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/store"
)

// File is one file a command writes: its store-relative path and full new contents.
type File struct {
	Path string
	Data []byte
}

// Paths returns the paths of fs, in order.
func Paths(fs []File) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Path
	}
	return out
}

// ValidationError is a write refused by §7 validation (exit 1): the error findings about
// the written entries and their link targets.
type ValidationError struct{ Findings []store.Finding }

func (e *ValidationError) Error() string {
	var b strings.Builder
	for i, f := range e.Findings {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s: %s %s", f.Path, f.Code, f.Message)
	}
	return b.String()
}

// Errors runs the §7 validation of tree and returns the errors about paths and the entries
// they link to, as `check --staged` selects them. Warnings never refuse a write.
func Errors(tree store.Tree, today time.Time, paths []string) ([]store.Finding, error) {
	fs, err := check.Run(tree, today, slices.Clone(paths))
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(fs, func(f store.Finding) bool { return !check.IsError(f) }), nil
}

// Validate is SPEC §9.1 step 3 for computed contents: the store as it would be with files
// written, validated before anything is written. Errors are a *ValidationError.
func Validate(root string, today time.Time, files []File) error {
	tree := &Pending{Base: store.WorkTree{Root: root}, Files: map[string][]byte{}}
	for _, f := range files {
		tree.Files[f.Path] = f.Data
	}
	errs, err := Errors(tree, today, Paths(files))
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return &ValidationError{errs}
	}
	return nil
}

// Write is SPEC §9.1 steps 4 and 5: write each file through a temporary file and a rename,
// read every one back, and validate the written entries. On failure the previous contents
// are restored (new files are deleted) and the error is returned; a validation failure is a
// *ValidationError.
func (s *Session) Write(today time.Time, files []File) error {
	for _, f := range files {
		p := filepath.Join(s.Root, filepath.FromSlash(f.Path))
		sv := saved{path: p, mode: 0o644}
		if info, err := os.Lstat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sv.exists, sv.data, sv.mode = true, data, info.Mode().Perm()
		}
		s.saved = append(s.saved, sv)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			s.restore()
			return err
		}
		if err := atomicWrite(p, f.Data, sv.mode); err != nil {
			s.restore()
			return err
		}
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(f.Path)))
		if err != nil || !bytes.Equal(got, f.Data) {
			s.restore()
			if err == nil {
				err = fmt.Errorf("%s: read back different bytes than were written", f.Path)
			}
			return err
		}
	}
	errs, err := Errors(store.WorkTree{Root: s.Root}, today, Paths(files))
	if err == nil && len(errs) > 0 {
		err = &ValidationError{errs}
	}
	if err != nil {
		s.restore()
		return err
	}
	return nil
}

// Check is SPEC §9.1 step 5 for `save`, which wrote nothing: validation only.
func (s *Session) Check(today time.Time, paths []string) error {
	errs, err := Errors(store.WorkTree{Root: s.Root}, today, paths)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return &ValidationError{errs}
	}
	return nil
}

// restore puts back what Write replaced, newest first.
func (s *Session) restore() {
	for i := len(s.saved) - 1; i >= 0; i-- {
		sv := s.saved[i]
		if sv.exists {
			atomicWrite(sv.path, sv.data, sv.mode)
		} else {
			os.Remove(sv.path)
		}
	}
	s.saved = nil
}

// atomicWrite writes data to p through a temporary file in the same directory, renamed into
// place, with the file and the directory synced (IMPLEMENTATION.md, "Atomic write").
func atomicWrite(p string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(p)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(name)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, p); err != nil {
		return err
	}
	ok = true
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Pending is the store's working tree with files not yet written laid over it, so that
// step 3 validates the contents a command is about to write.
type Pending struct {
	Base  store.Tree
	Files map[string][]byte
}

// Children implements store.Tree.
func (t *Pending) Children(dir string) ([]store.Item, error) {
	items, err := t.Base.Children(dir)
	if err != nil && !store.IsNotExist(err) {
		return nil, err
	}
	found := err == nil
	names := map[string]bool{}
	for _, it := range items {
		names[it.Name] = true
	}
	prefix := dir + "/"
	if dir == "" {
		prefix = ""
	}
	for p := range t.Files {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		found = true
		name, _, isDir := strings.Cut(rest, "/")
		if names[name] {
			continue
		}
		names[name] = true
		items = append(items, store.Item{Name: name, Dir: isDir, Regular: !isDir})
	}
	if !found {
		return nil, err
	}
	slices.SortFunc(items, func(a, b store.Item) int { return strings.Compare(a.Name, b.Name) })
	return items, nil
}

// ReadFile implements store.Tree.
func (t *Pending) ReadFile(p string) ([]byte, error) {
	if data, ok := t.Files[p]; ok {
		return data, nil
	}
	return t.Base.ReadFile(p)
}

// Locate implements store.Tree.
func (t *Pending) Locate(p string) store.Location {
	if _, ok := t.Files[path.Clean(p)]; ok {
		return store.Inside
	}
	return t.Base.Locate(p)
}
