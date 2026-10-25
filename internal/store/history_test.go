package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arrowheadmind/memex/internal/gitx"
)

func TestReadHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	setGitEnv(t, t.TempDir())
	g := gitx.New(root, 0)
	run := func(args ...string) {
		t.Helper()
		if _, err := g.Run(args...); err != nil {
			t.Fatal(err)
		}
	}
	put := func(p, data string) {
		t.Helper()
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	h, err := ReadHistory(g, []string{"tasks"})
	if err != nil || len(h.Versions) != 0 {
		t.Fatalf("no commits: %v, %v", h, err)
	}
	put("tasks/a.md", "a1")
	put("tasks/b.md", "b1")
	put("rules/r.md", "r1")
	run("add", "-A")
	run("commit", "-q", "-m", "1")
	put("tasks/a.md", "a2")
	run("commit", "-q", "-am", "2")
	run("rm", "-q", "tasks/b.md")
	run("commit", "-q", "-m", "3")
	put("tasks/b.md", "b2")
	put("tasks/c.md", "c1")
	run("add", "-A")
	run("commit", "-q", "-m", "4")
	run("rm", "-q", "tasks/c.md")
	run("commit", "-q", "-m", "5")
	h, err = ReadHistory(g, []string{"tasks"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"tasks/a.md": {"a2", "a1"}, "tasks/b.md": {"b2"}}
	if len(h.Versions) != len(want) {
		t.Errorf("paths %v", h.Versions)
	}
	for p, vs := range want {
		got := h.Versions[p]
		if len(got) != len(vs) {
			t.Errorf("%s: %q, want %q", p, got, vs)
			continue
		}
		for i := range vs {
			if string(got[i]) != vs[i] {
				t.Errorf("%s[%d] = %q, want %q", p, i, got[i], vs[i])
			}
		}
	}
}

// A file version the log names but the repository does not hold is "object <oid> is
// missing" (SPEC §7, summary currency); a git error is git's first error line.
func TestReadHistoryFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	setGitEnv(t, t.TempDir())
	g := gitx.New(root, 0)
	run := func(args ...string) string {
		t.Helper()
		out, err := g.Run(args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run("init", "-q")
	os.MkdirAll(filepath.Join(root, "tasks"), 0o755)
	os.WriteFile(filepath.Join(root, "tasks/a.md"), []byte("a1"), 0o644)
	run("add", "-A")
	run("commit", "-q", "-m", "1")
	oid := strings.TrimSpace(run("rev-parse", "HEAD:tasks/a.md"))
	if err := os.Remove(filepath.Join(root, ".git", "objects", oid[:2], oid[2:])); err != nil {
		t.Fatal(err)
	}
	_, err := ReadHistory(g, []string{"tasks"})
	if err == nil || HistoryReason(err, 0) != "object "+oid+" is missing" {
		t.Errorf("missing blob: %v", err)
	}
	ge := &gitx.Error{Args: []string{"log"}, ExitCode: 128, Stderr: "warning: x\nerror: Object 1 not a commit\nfatal: cannot simplify\n"}
	if r := HistoryReason(ge, 0); r != "error: Object 1 not a commit" {
		t.Errorf("git error: %q", r)
	}
	if r := HistoryReason(&gitx.Error{ExitCode: 1}, 0); r != "git exited with status 1" {
		t.Errorf("no stderr: %q", r)
	}
	if r := HistoryReason(&gitx.Error{TimedOut: true}, 10*time.Second); r != "timed out after 10s" {
		t.Errorf("timeout: %q", r)
	}
}
