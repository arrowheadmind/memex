package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arrowheadmind/memex/internal/gitx"
)

// The last change of a path is the committer date of the newest commit that changes it, in
// the offset that commit records (SPEC §7, W-OPEN-STALE): a commit at 23:30 at -05:00 is
// dated that day, not the next one in UTC. A commit that only touches another path does not
// move it, and the author date is not read.
func TestReadHistoryChanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	setGitEnv(t, t.TempDir())
	g := gitx.New(root, 0)
	commit := func(committer, author string, files map[string]string) {
		t.Helper()
		for p, data := range files {
			os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
			if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("GIT_COMMITTER_DATE", committer)
		t.Setenv("GIT_AUTHOR_DATE", author)
		g := gitx.New(root, 0) // gitx reads the environment when it is made
		for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "c"}} {
			if _, err := g.Run(args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := g.Run("init", "-q"); err != nil {
		t.Fatal(err)
	}
	commit("2026-01-01T23:30:00-05:00", "2025-06-01T12:00:00+00:00", map[string]string{"tasks/a.md": "a1", "tasks/b.md": "b1"})
	commit("2026-01-05T00:30:00+02:00", "2025-06-01T12:00:00+00:00", map[string]string{"tasks/b.md": "b2"})
	commit("2026-01-09T12:00:00+00:00", "2026-01-09T12:00:00+00:00", map[string]string{"rules/r.md": "r1"})
	h, err := ReadHistory(g, []string{"tasks"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"tasks/a.md": "2026-01-01", "tasks/b.md": "2026-01-05"}
	if len(h.Changed) != len(want) {
		t.Errorf("Changed = %v, want %v", h.Changed, want)
	}
	for p, d := range want {
		if h.Changed[p] != d {
			t.Errorf("Changed[%s] = %q, want %q", p, h.Changed[p], d)
		}
	}
}

// stale_days (SPEC §4): 0 to 36500, default 14; anything else is E-MANIFEST.
func TestManifestStaleDays(t *testing.T) {
	cases := []struct {
		toml    string
		days    int64
		finding string
	}{
		{"format = 1\n", 14, ""},
		{"format = 1\nstale_days = 0\n", 0, ""},
		{"format = 1\nstale_days = 36500\n", 36500, ""},
		{"format = 1\nstale_days = 36501\n", 14, "stale_days must be an integer from 0 to 36500"},
		{"format = 1\nstale_days = -1\n", 14, "stale_days must be an integer from 0 to 36500"},
		{"format = 1\nstale_days = \"7\"\n", 14, "stale_days must be an integer from 0 to 36500"},
		{"format = 1\nstale_day = 7\n", 14, `unknown key "stale_day" (did you mean "stale_days"?)`},
	}
	for _, c := range cases {
		m, fs, err := ParseManifest([]byte(c.toml))
		if err != nil {
			t.Fatalf("%q: %v", c.toml, err)
		}
		if m.StaleDays != c.days {
			t.Errorf("%q: StaleDays = %d, want %d", c.toml, m.StaleDays, c.days)
		}
		got := ""
		if len(fs) > 0 {
			got = fs[0].Message
		}
		if got != c.finding || len(fs) > 1 {
			t.Errorf("%q: findings %v, want %q", c.toml, fs, c.finding)
		}
	}
}
