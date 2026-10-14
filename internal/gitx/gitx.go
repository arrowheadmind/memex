// Package gitx runs the git executable for memex, with the environment of SPEC §3.3.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// scrubbed are the variables removed from git's environment, so that memex called from
// another repository's git hook still addresses the store (SPEC §3.3).
var scrubbed = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
}

// environ returns the environment given to git: base (KEY=VALUE pairs) with the scrub list
// removed, except GIT_INDEX_FILE when keepIndex is set (`check --staged`), and with
// GIT_TERMINAL_PROMPT=0, GIT_EDITOR=true, GIT_SSH_COMMAND in batch mode and LC_ALL=C.
func environ(base []string, keepIndex bool) []string {
	out := make([]string, 0, len(base)+4)
	ssh := ""
	for _, kv := range base {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "GIT_INDEX_FILE" && keepIndex:
			out = append(out, kv)
		case isScrubbed(k), k == "GIT_TERMINAL_PROMPT", k == "GIT_EDITOR", k == "LC_ALL":
		case k == "GIT_SSH_COMMAND":
			ssh = v
		default:
			out = append(out, kv)
		}
	}
	if ssh == "" {
		ssh = "ssh"
	}
	return append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
		"GIT_SSH_COMMAND="+ssh+" -o BatchMode=yes",
		"LC_ALL=C",
	)
}

func isScrubbed(k string) bool {
	for _, s := range scrubbed {
		if k == s {
			return true
		}
	}
	return false
}

// Git runs git in one repository, always with the environment of SPEC §3.3. New is the only
// way to make one.
type Git struct {
	dir     string
	env     []string
	timeout time.Duration
	ctx     context.Context // cancelled to interrupt a command; Background when not given
}

// Option changes how New builds a Git.
type Option func(*options)

type options struct {
	staged bool
	extra  []string
	ctx    context.Context
}

// ErrInterrupted is the cancellation cause that marks an interruption by a signal: the
// running git is sent SIGTERM, so that it removes its lock files, rather than SIGKILL.
var ErrInterrupted = errors.New("interrupted")

// Context makes every command stop when ctx is cancelled. A cancellation whose cause is
// ErrInterrupted is reported as Error.Interrupted.
func Context(ctx context.Context) Option { return func(o *options) { o.ctx = ctx } }

// Staged keeps GIT_INDEX_FILE, for `check --staged` only (SPEC §3.3).
func Staged() Option { return func(o *options) { o.staged = true } }

// Extra adds KEY=VALUE pairs to the environment, after the §3.3 changes.
func Extra(kv ...string) Option { return func(o *options) { o.extra = append(o.extra, kv...) } }

// New returns a Git for the repository at dir. Every command runs with the caller's
// environment changed as SPEC §3.3 says; a timeout of zero means none.
func New(dir string, timeout time.Duration, opts ...Option) *Git {
	var o options
	for _, f := range opts {
		f(&o)
	}
	if o.ctx == nil {
		o.ctx = context.Background()
	}
	return &Git{dir: dir, env: append(environ(os.Environ(), o.staged), o.extra...), timeout: timeout, ctx: o.ctx}
}

// Error is a failed git command: its arguments, exit status and captured stderr.
type Error struct {
	Args        []string
	ExitCode    int // -1 when git did not exit normally
	Stderr      string
	TimedOut    bool
	Interrupted bool
	Err         error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if e.TimedOut {
		msg = "timed out"
	} else if e.Interrupted {
		msg = "interrupted"
	} else if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

// Run runs git with args and returns its standard output. When the timeout expires the whole
// process group is killed, so that an ssh child does not linger.
func (g *Git) Run(args ...string) (string, error) {
	return g.RunInput(nil, args...)
}

// RunInput is Run with stdin as git's standard input.
func (g *Git) RunInput(stdin []byte, args ...string) (string, error) {
	ctx := g.ctx
	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	cmd.Env = g.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		sig := syscall.SIGKILL
		if errors.Is(context.Cause(g.ctx), ErrInterrupted) {
			sig = syscall.SIGTERM // git removes its lock files on SIGTERM
		}
		err := syscall.Kill(-cmd.Process.Pid, sig)
		if errors.Is(err, syscall.ESRCH) { // the group has already exited
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 5 * time.Second
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	err := cmd.Run()
	if err == nil {
		return out.String(), nil
	}
	e := &Error{Args: args, ExitCode: -1, Stderr: errb.String(), Err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		e.ExitCode = exitErr.ExitCode()
	}
	switch {
	case errors.Is(context.Cause(g.ctx), ErrInterrupted):
		e.Interrupted = true
	case ctx.Err() == context.DeadlineExceeded:
		e.TimedOut = true
	}
	return out.String(), e
}
