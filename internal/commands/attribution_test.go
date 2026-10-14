package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestAttribution checks SPEC §5.5: a linked worktree is attributed to its repository, and
// the store itself, or a directory outside any repository, to nothing.
func TestAttribution(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gc := filepath.Join(dir, "gitconfig")
	os.WriteFile(gc, []byte("[user]\n\tname = test\n\temail = test@example.invalid\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", gc)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo, st, outside := filepath.Join(dir, "app"), filepath.Join(dir, "store"), filepath.Join(dir, "plain")
	for _, d := range []string{repo, st, outside} {
		os.MkdirAll(d, 0o755)
	}
	gitT(t, repo, "init", "-q")
	gitT(t, repo, "commit", "-q", "--allow-empty", "-m", "x")
	gitT(t, repo, "worktree", "add", "-q", filepath.Join(repo, ".worktrees", "task"))
	gitT(t, st, "init", "-q")
	os.MkdirAll(filepath.Join(st, "rules"), 0o755)
	cases := []struct {
		cwd  string
		want []string
	}{
		{repo, []string{"app"}},
		{filepath.Join(repo, ".worktrees", "task"), []string{"app"}},
		{filepath.Join(st, "rules"), nil},
		{outside, nil},
	}
	for _, c := range cases {
		t.Chdir(c.cwd)
		if got := attribution(st); !slices.Equal(got, c.want) {
			t.Errorf("attribution in %s = %q, want %q", c.cwd, got, c.want)
		}
	}
}
