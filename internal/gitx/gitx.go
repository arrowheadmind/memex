// Package gitx runs the git executable for memex, with the environment of SPEC §3.3.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// Env returns the environment given to git: base (KEY=VALUE pairs) with the scrub list
// removed, except GIT_INDEX_FILE when keepIndex is set (`check --staged`), and with
// GIT_TERMINAL_PROMPT=0, GIT_EDITOR=true, GIT_SSH_COMMAND in batch mode and LC_ALL=C.
func Env(base []string, keepIndex bool) []string {
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

// Git runs git in one repository.
type Git struct {
	Dir string   // working directory, the store root
	Env []string // the result of Env
}

// Error is a failed git command: its arguments, exit status and captured stderr.
type Error struct {
	Args     []string
	ExitCode int // -1 when git did not exit normally
	Stderr   string
	TimedOut bool
	Err      error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if e.TimedOut {
		msg = "timed out"
	} else if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

// Run runs git with args and returns its standard output. A timeout of zero means none;
// otherwise the whole process group is killed when it expires, so that an ssh child does not
// linger.
func (g *Git) Run(timeout time.Duration, args ...string) (string, error) {
	return g.RunInput(timeout, nil, args...)
}

// RunInput is Run with stdin as git's standard input.
func (g *Git) RunInput(timeout time.Duration, stdin []byte, args ...string) (string, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	cmd.Env = g.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
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
	if ctx.Err() == context.DeadlineExceeded {
		e.TimedOut = true
	}
	return out.String(), e
}
