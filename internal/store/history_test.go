package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
