package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// writing is a mutating command between the lock and the report (SPEC §9.1).
type writing struct {
	e     *Env
	cfg   *config.Config
	today time.Time
	root  string
	s     *write.Session
}

// beginWrite opens the store and takes the lock; with freshen, it also runs step 2. On
// failure it returns the exit code. Every caller but save needs memex.toml committed.
func (e *Env) beginWrite(freshen bool) (*writing, int, bool) {
	return e.beginWriteFor(freshen, true)
}

func (e *Env) beginWriteFor(freshen, needCommitted bool) (*writing, int, bool) {
	cfg, today, root, err := e.openStore()
	if err != nil {
		return nil, e.failErr(err), false
	}
	if code, ok := e.checkManifest(root); !ok {
		return nil, code, false
	}
	if needCommitted {
		if code, ok := e.checkCommitted(root); !ok {
			return nil, code, false
		}
	}
	s, err := write.Begin(root, cfg.NetworkTimeout, write.LockWait)
	if err != nil {
		return nil, e.beginFailed(err), false
	}
	w := &writing{e, cfg, today, root, s}
	if err := s.CheckInProgress(); err != nil {
		return nil, w.refuse(spec.ExitFailure, "%s", err.Error()), false
	}
	if freshen {
		s.Freshen()
	}
	if s.Interrupted() {
		return nil, w.refuse(spec.ExitFailure, "%s", write.ErrInterrupted.Error()), false
	}
	return w, 0, true
}

// checkManifest refuses a store whose format this memex cannot read (exit 2, SPEC §4).
func (e *Env) checkManifest(root string) (int, bool) {
	if _, _, err := store.ReadManifest(store.WorkTree{Root: root}); err != nil {
		var fe *store.FormatError
		if errors.As(err, &fe) {
			return e.fail(spec.ExitUsage, "%s", fe.Msg), false
		}
		panic(err)
	}
	return 0, true
}

// beginFailed reports a session that could not start: a lock held by another memex
// (exit 4), an interruption while waiting, or a lock file that cannot be written (exit 1).
func (e *Env) beginFailed(err error) int {
	var le *write.LockError
	var we *write.WriteError
	switch {
	case errors.As(err, &le):
		return e.fail(spec.ExitLock, "%s", le.Error())
	case errors.Is(err, write.ErrInterrupted), errors.As(err, &we):
		return e.fail(spec.ExitFailure, "%s", err.Error())
	}
	panic(err)
}

// checkCommitted refuses a write to a store whose memex.toml is not in HEAD, such as one
// that init has just created: its pre-commit hook would refuse the commit (SPEC §9.1 step 3).
func (e *Env) checkCommitted(root string) (int, bool) {
	if _, err := gitx.New(root, 0).Run("cat-file", "-e", "HEAD:"+store.ManifestFile); err != nil {
		return e.fail(spec.ExitFailure, "store has no commits yet; run memex save"), false
	}
	return 0, true
}

// entries reads the store's entries after freshening, reporting the unreadable ones.
func (w *writing) entries() []*store.Entry {
	entries, skipped, err := store.LoadEntries(store.WorkTree{Root: w.root})
	if err != nil {
		panic(err)
	}
	for _, s := range skipped {
		fmt.Fprintf(w.e.Stderr, "memex: skipped %s: %s; run memex check\n", s.Path, s.Problem)
	}
	return entries
}

// end releases the lock and reports a fetch that failed while freshening, unless the
// outcome already names it.
func (w *writing) end(o *write.Outcome) {
	w.s.End()
	if w.s.FetchErr != "" && (o == nil || !strings.HasSuffix(o.Line, w.s.FetchErr)) {
		fmt.Fprintf(w.e.Stderr, "memex: fetch failed: %s\n", w.s.FetchErr)
	}
}

// refuseUncommitted refuses to rewrite an entry that differs from HEAD (SPEC §11.7, §11.8):
// the rewrite would commit someone's unsaved edit with it.
func (w *writing) refuseUncommitted(path string) (int, bool) {
	if w.s.Uncommitted(path) {
		return w.refuse(spec.ExitFailure, "%s has uncommitted changes; run memex save first", path), true
	}
	return 0, false
}

// refuse ends a write that changed nothing with an error message.
func (w *writing) refuse(code int, format string, args ...any) int {
	w.end(nil)
	return w.e.fail(code, format, args...)
}

// failed ends a write whose Write or Commit failed. Nothing is left changed.
func (w *writing) failed(err error) int {
	w.end(nil)
	var ve *write.ValidationError
	var ie *write.IndexLockError
	var ge *write.GitError
	switch {
	case errors.As(err, &ve):
		return w.e.validationFailed(ve)
	case errors.As(err, &ie):
		return w.e.fail(spec.ExitLock, "%s; nothing was changed", ie.Error())
	case errors.As(err, &ge):
		return w.e.fail(spec.ExitFailure, "%s", ge.Error())
	case errors.Is(err, write.ErrInterrupted), errors.As(err, new(*write.WriteError)), errors.As(err, new(*write.ChangedError)):
		return w.e.fail(spec.ExitFailure, "%s", err.Error())
	}
	panic(err)
}

// validationFailed prints every error finding of a refused write (exit 1).
func (e *Env) validationFailed(ve *write.ValidationError) int {
	for _, f := range ve.Findings {
		fmt.Fprintf(e.Stderr, "memex: %s: %s %s\n", f.Path, f.Code, f.Message)
	}
	return e.fail(spec.ExitFailure, "refused: the result would not validate; nothing was changed")
}

// commit writes files, commits them with msg and prints lines, then the outcome (SPEC §9.1
// steps 4 to 8).
func (w *writing) commit(files []write.File, msg string, lines ...string) int {
	if err := w.s.Write(w.today, files); err != nil {
		return w.failed(err)
	}
	return w.commitPaths(write.Paths(files), msg, lines...)
}

// commitPaths commits paths already in the working tree (steps 6 to 8).
func (w *writing) commitPaths(paths []string, msg string, lines ...string) int {
	o, err := w.s.Commit(paths, msg)
	if err != nil {
		return w.failed(err)
	}
	for _, l := range lines {
		fmt.Fprintln(w.e.Stdout, l)
	}
	fmt.Fprintln(w.e.Stdout, o.Line)
	w.end(&o)
	return o.Code
}

// nothing reports a write that had nothing to commit, after lines.
func (w *writing) nothing(lines ...string) int {
	for _, l := range lines {
		fmt.Fprintln(w.e.Stdout, l)
	}
	o := write.NothingToCommit
	fmt.Fprintln(w.e.Stdout, o.Line)
	w.end(&o)
	return o.Code
}
