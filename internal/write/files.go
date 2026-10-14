package write

import (
	"bytes"
	"errors"
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

// File is one file a command writes: its store-relative path and full new contents. With
// Check set, the file must still hold Expect when it is written (nil Expect: must not
// exist), or the write stops with a *ChangedError.
type File struct {
	Path   string
	Data   []byte
	Check  bool
	Expect []byte
}

// WriteError is a file that could not be written (exit 1). Err is the operating system's
// error; the message names only the store-relative path.
type WriteError struct {
	Path string
	Err  error
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("cannot write %s: %s; nothing was changed", e.Path, osReason(e.Err))
}

// osReason is the operating system's text for err, without the absolute paths that
// *os.PathError and *os.LinkError add.
func osReason(err error) string {
	var pe *os.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		return pe.Err.Error()
	case errors.As(err, &le):
		return le.Err.Error()
	}
	return err.Error()
}

// ChangedError is a file that changed between the command reading it and writing it
// (exit 1).
type ChangedError struct{ Path string }

func (e *ChangedError) Error() string {
	return e.Path + " changed while memex was working; retry"
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
	fail := func(err error) error {
		s.restore()
		return err
	}
	for _, f := range files {
		p := filepath.Join(s.Root, filepath.FromSlash(f.Path))
		sv := saved{path: p, mode: 0o644}
		cur, err := os.ReadFile(p)
		info, statErr := os.Lstat(p)
		switch {
		case err == nil && statErr == nil:
			sv.exists, sv.data, sv.mode = true, cur, info.Mode().Perm()
		case !errors.Is(err, os.ErrNotExist):
			return fail(&WriteError{f.Path, err})
		}
		if f.Check && (sv.exists != (f.Expect != nil) || !bytes.Equal(cur, f.Expect)) {
			return fail(&ChangedError{f.Path})
		}
		if err := s.mkdirs(filepath.Dir(p)); err != nil {
			return fail(&WriteError{f.Path, err})
		}
		s.saved = append(s.saved, sv)
		if err := atomicWrite(p, f.Data, sv.mode); err != nil {
			return fail(&WriteError{f.Path, err})
		}
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(f.Path)))
		if err != nil {
			return fail(&WriteError{f.Path, err})
		}
		if !bytes.Equal(got, f.Data) {
			return fail(&WriteError{f.Path, errors.New("read back different bytes than were written")})
		}
	}
	errs, err := Errors(store.WorkTree{Root: s.Root}, today, Paths(files))
	if err == nil && len(errs) > 0 {
		err = &ValidationError{errs}
	}
	if err != nil {
		return fail(err)
	}
	return nil
}

// mkdirs creates dir and its missing parents, recording each one so that restore can
// remove them again.
func (s *Session) mkdirs(dir string) error {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil || d == s.Root || d == filepath.Dir(d) {
			break
		}
		missing = append(missing, d)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil {
			return err
		}
		s.created = append(s.created, missing[i])
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

// Restore puts back what Write replaced, for a command that refuses after writing.
func (s *Session) Restore() { s.restore() }

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
	for i := len(s.created) - 1; i >= 0; i-- {
		os.Remove(s.created[i]) // only when empty
	}
	s.saved, s.created = nil, nil
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
