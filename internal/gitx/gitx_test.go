package gitx

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEnv(t *testing.T) {
	base := []string{
		"HOME=/h", "GIT_DIR=/other/.git", "GIT_WORK_TREE=/other", "GIT_INDEX_FILE=/tmp/idx",
		"GIT_COMMON_DIR=x", "GIT_PREFIX=p/", "GIT_OBJECT_DIRECTORY=o",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=a", "LC_ALL=de_DE.UTF-8", "GIT_EDITOR=vim",
		"GIT_TERMINAL_PROMPT=1", "GIT_AUTHOR_NAME=kept",
	}
	want := []string{"HOME=/h", "GIT_AUTHOR_NAME=kept", "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes", "LC_ALL=C"}
	if got := environ(base, false); !slices.Equal(got, want) {
		t.Errorf("environ = %q\nwant  %q", got, want)
	}
	got := environ(append(base, "GIT_SSH_COMMAND=ssh -i key"), true)
	if !slices.Contains(got, "GIT_INDEX_FILE=/tmp/idx") || !slices.Contains(got, "GIT_SSH_COMMAND=ssh -i key -o BatchMode=yes") {
		t.Errorf("environ(keepIndex) = %q", got)
	}
}

func TestRunTimeout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("PATH", pathOf(t))
	t.Setenv("GIT_DIR", "/nonexistent/.git") // scrubbed by New
	dir := t.TempDir()
	g := New(dir, 0)
	out, err := g.Run("--version")
	if err != nil || !strings.HasPrefix(out, "git version") {
		t.Fatalf("git --version: %q, %v", out, err)
	}
	_, err = g.Run("rev-parse", "HEAD")
	var ge *Error
	if !errors.As(err, &ge) || ge.ExitCode == 0 || !strings.Contains(ge.Stderr, "not a git repository") {
		t.Errorf("rev-parse outside a repository: %v", err)
	}
	// An alias that runs a shell sleeping longer than the timeout.
	start := time.Now()
	_, err = New(dir, 200*time.Millisecond).Run("-c", "alias.nap=!sleep 5", "nap")
	if !errors.As(err, &ge) || !ge.TimedOut || time.Since(start) > 3*time.Second {
		t.Errorf("timeout: %v after %v", err, time.Since(start))
	}
}

func pathOf(t *testing.T) string {
	git, _ := exec.LookPath("git")
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not on PATH")
	}
	sh, _ := exec.LookPath("sh")
	dirs := []string{dir(git), dir(sleep), dir(sh)}
	return strings.Join(dirs, ":")
}

func dir(p string) string { return p[:strings.LastIndex(p, "/")] }

func TestReason(t *testing.T) {
	for in, want := range map[string]string{
		"From x\nfatal: no repo\nmore\n":   "fatal: no repo",
		"hint: a\nerror: failed to push\n": "error: failed to push",
		"\n  plain line  \nnext\n":         "plain line",
		"":                                 "",
	} {
		if got := Reason(in); got != want {
			t.Errorf("Reason(%q) = %q, want %q", in, got, want)
		}
	}
}
