package write

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/spec"
)

// Outcome is the last line of a mutating command and its exit code (SPEC §9.4).
type Outcome struct {
	Line string
	Code int
}

// NothingToCommit is the outcome of a write that changed nothing.
var NothingToCommit = Outcome{"nothing to commit", spec.ExitOK}

// IndexLockError is a git index that stayed locked by another process through every retry
// (exit 4).
type IndexLockError struct{ Err *gitx.Error }

func (e *IndexLockError) Error() string {
	return "the git index is locked by another process: " + firstLine(e.Err.Stderr)
}

// GitError is a git command of the write that failed, such as a commit the pre-commit hook
// refused (exit 1). The files written have been restored.
type GitError struct{ Err *gitx.Error }

func (e *GitError) Error() string {
	msg := strings.TrimRight(e.Err.Stderr, "\n")
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s failed; nothing was changed:\n%s", e.Err.Args[0], msg)
}

// indexLockDelays are the retries of SPEC §9.1 for an index locked by another process.
var indexLockDelays = []time.Duration{200 * time.Millisecond, 400 * time.Millisecond,
	800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond}

// Session is one run of the write protocol, from the lock to the report.
type Session struct {
	Root     string
	FetchErr string // why the freshening fetch failed, or ""

	git     *gitx.Git // local commands, unbounded
	net     *gitx.Git // fetch, pull and push, bounded by network_timeout
	timeout int       // network_timeout in seconds
	lockF   *os.File
	up      *upstream
	saved   []saved
	index   []byte // the git index before Commit staged anything; nil when there was none
	sleep   func(time.Duration)
}

type upstream struct {
	branch, remote, merge string
}

type saved struct {
	path   string
	exists bool
	data   []byte
	mode   os.FileMode
}

// Begin takes the lock of SPEC §9.1 step 1, waiting up to wait. timeout is network_timeout
// in seconds. A lock that stays held is a *LockError.
func Begin(root string, timeout int, wait time.Duration) (*Session, error) {
	f, err := lock(root, wait)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Root:    root,
		git:     gitx.New(root, 0),
		net:     gitx.New(root, time.Duration(timeout)*time.Second),
		timeout: timeout,
		lockF:   f,
		sleep:   time.Sleep,
	}
	s.up = s.findUpstream()
	return s, nil
}

// End releases the lock (SPEC §9.1 step 8).
func (s *Session) End() {
	if s.lockF != nil {
		s.lockF.Close()
		s.lockF = nil
	}
}

// findUpstream returns the current branch's upstream, or nil when there is none.
func (s *Session) findUpstream() *upstream {
	out, err := s.git.Run("symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		return nil
	}
	u := &upstream{branch: strings.TrimSpace(out)}
	if out, err = s.git.Run("config", "branch."+u.branch+".remote"); err != nil {
		return nil
	}
	u.remote = strings.TrimSpace(out)
	if out, err = s.git.Run("config", "branch."+u.branch+".merge"); err != nil {
		return nil
	}
	u.merge = strings.TrimSpace(out)
	if u.remote == "" || u.merge == "" {
		return nil
	}
	return u
}

// HasUpstream reports whether the current branch has an upstream.
func (s *Session) HasUpstream() bool { return s.up != nil }

// reason is the outcome reason for a failed network command (SPEC §9.4): the timeout, or
// git's first line of standard error.
func (s *Session) reason(err error) string {
	var ge *gitx.Error
	if errors.As(err, &ge) {
		if ge.TimedOut {
			return fmt.Sprintf("timed out after %ds", s.timeout)
		}
		if l := firstLine(ge.Stderr); l != "" {
			return l
		}
	}
	return err.Error()
}

func firstLine(text string) string {
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// run runs a local git command that may need the index lock, retrying while another
// process holds it (SPEC §9.1). An index that stays locked is an *IndexLockError.
func (s *Session) run(g *gitx.Git, args ...string) (string, error) {
	for i := 0; ; i++ {
		out, err := g.Run(args...)
		var ge *gitx.Error
		if err == nil || !errors.As(err, &ge) || !strings.Contains(ge.Stderr, "index.lock': File exists") {
			return out, err
		}
		if i == len(indexLockDelays) {
			return out, &IndexLockError{ge}
		}
		s.sleep(indexLockDelays[i])
	}
}

// Freshen is SPEC §9.1 step 2: fetch the upstream, and fast-forward when the branch is
// strictly behind and no tracked file has uncommitted changes. A failed fetch is recorded
// in FetchErr; nothing here stops the write.
func (s *Session) Freshen() {
	if s.up == nil {
		return
	}
	if err := s.fetch(); err != nil {
		s.FetchErr = s.reason(err)
		return
	}
	ahead, behind, err := s.counts()
	if err != nil || behind == 0 || ahead > 0 || s.dirty() || s.rebaseInProgress() {
		return
	}
	s.run(s.git, "merge", "--ff-only", "-q", "@{u}")
}

func (s *Session) fetch() error {
	_, err := s.net.Run("fetch", "-q", s.up.remote)
	return err
}

// counts returns how many commits HEAD is ahead of and behind its upstream.
func (s *Session) counts() (ahead, behind int, err error) {
	out, err := s.git.Run("rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, fmt.Errorf("git rev-list: unexpected output %q", out)
	}
	ahead, _ = strconv.Atoi(f[0])
	behind, _ = strconv.Atoi(f[1])
	return ahead, behind, nil
}

// dirty reports whether a tracked file has uncommitted changes, staged or not.
func (s *Session) dirty() bool {
	out, err := s.git.Run("status", "--porcelain", "-z", "--untracked-files=no")
	return err != nil || out != ""
}

func (s *Session) rebaseInProgress() bool {
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		p, err := s.git.Run("rev-parse", "--git-path", d)
		if err != nil {
			continue
		}
		p = strings.TrimSpace(p)
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.Root, p)
		}
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// conflicted lists the unmerged paths, in code-point order, separated by ", ".
func (s *Session) conflicted() string {
	out, _ := s.git.Run("diff", "--name-only", "--diff-filter=U", "-z")
	var ps []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			ps = append(ps, p)
		}
	}
	slices.Sort(ps)
	return strings.Join(slices.Compact(ps), ", ")
}

func (s *Session) head7() string {
	out, err := s.git.Run("rev-parse", "HEAD")
	if err != nil || len(out) < 7 {
		return "0000000"
	}
	return out[:7]
}

func (s *Session) hasHead() bool {
	_, err := s.git.Run("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// Commit is SPEC §9.1 steps 6 and 7: stage and commit exactly paths with msg, then
// integrate with the upstream. When the commit fails, the files written by Write and the
// index are restored and the error is returned (*IndexLockError or *GitError).
func (s *Session) Commit(paths []string, msg string) (Outcome, error) {
	if err := s.saveIndex(); err != nil {
		return Outcome{}, err
	}
	fail := func(err error) (Outcome, error) {
		s.restore()
		s.restoreIndex()
		var ge *gitx.Error
		if errors.As(err, &ge) && !errors.As(err, new(*IndexLockError)) {
			err = &GitError{ge}
		}
		return Outcome{}, err
	}
	var add []string
	for _, p := range paths {
		if s.addable(p) {
			add = append(add, p)
		}
	}
	if len(add) > 0 {
		if _, err := s.run(s.git, append([]string{"add", "-A", "--"}, add...)...); err != nil {
			return fail(err)
		}
	}
	if _, err := s.run(s.git, append([]string{"commit", "-q", "-m", msg, "--"}, paths...)...); err != nil {
		return fail(err)
	}
	s.saved = nil
	return s.integrate(), nil
}

// addable reports whether git add can name p: it exists in the working tree or the index.
// A deletion already staged needs no add.
func (s *Session) addable(p string) bool {
	if _, err := os.Lstat(filepath.Join(s.Root, filepath.FromSlash(p))); err == nil {
		return true
	}
	out, err := s.git.Run("ls-files", "-z", "--", p)
	return err == nil && out != ""
}

func (s *Session) indexPath() (string, error) {
	p, err := s.git.Run("rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	p = strings.TrimSpace(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Root, p)
	}
	return p, nil
}

// saveIndex keeps a copy of the git index, so that a failed commit leaves the index as it
// was found.
func (s *Session) saveIndex() error {
	p, err := s.indexPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		s.index = nil
		return nil
	}
	s.index = data
	return err
}

func (s *Session) restoreIndex() {
	p, err := s.indexPath()
	if err != nil {
		return
	}
	if s.index == nil {
		os.Remove(p)
		return
	}
	atomicWrite(p, s.index, 0o644)
}

// integrate is SPEC §9.1 step 7.
func (s *Session) integrate() Outcome {
	sha := s.head7()
	notPushed := func(reason string) Outcome {
		return Outcome{fmt.Sprintf("committed %s, NOT pushed: %s", s.head7(), reason), spec.ExitNotPushed}
	}
	switch {
	case s.up == nil:
		return Outcome{fmt.Sprintf("committed %s, no upstream", sha), spec.ExitOK}
	case s.rebaseInProgress():
		return notPushed("rebase in progress")
	case s.dirty():
		return notPushed("uncommitted changes (run memex save)")
	case s.FetchErr != "":
		return notPushed(s.FetchErr)
	}
	var last string
	for attempt := 0; attempt < MaxPushAttempts; attempt++ {
		if _, err := s.run(s.net, "pull", "--rebase", "--no-autostash", "-q"); err != nil {
			if s.rebaseInProgress() {
				paths := s.conflicted()
				s.run(s.git, "rebase", "--abort")
				return Outcome{fmt.Sprintf("committed %s, NOT integrated: rebase conflict in %s", s.head7(), paths), spec.ExitConflict}
			}
			return notPushed(s.reason(err))
		}
		rejected, err := s.push()
		if err == nil {
			return Outcome{"pushed " + s.head7(), spec.ExitOK}
		}
		last = s.reason(err)
		if !rejected {
			break
		}
	}
	return notPushed(last)
}

// MaxPushAttempts bounds the pull-and-push loop of SPEC §9.1 step 7.
const MaxPushAttempts = 5

// push pushes HEAD to the upstream branch. rejected reports a non-fast-forward rejection.
func (s *Session) push() (rejected bool, err error) {
	out, err := s.net.Run("push", "--porcelain", s.up.remote, "HEAD:"+s.up.merge)
	if err == nil {
		return false, nil
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "[rejected]") && (strings.Contains(l, "(fetch first)") || strings.Contains(l, "(non-fast-forward)")) {
			return true, err
		}
	}
	return false, err
}
