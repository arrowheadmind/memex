package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/arrowheadmind/memex/internal/gitx"
)

const okBody = "This body is long enough to pass the content measure of eighty code points, easily so.\n"

func entryText(title, links string) string {
	return "+++\ntitle = \"" + title + "\"\nstatus = \"active\"\ncreated = 2026-01-10\nsummary = \"S\"\ntags = [\"t\"]\n+++\n\n" + okBody + links
}

// TestCheckStaged runs check --staged against a real repository: only staged entries and
// their link targets are validated, from the index contents, and GIT_INDEX_FILE is honoured.
func TestCheckStaged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	environ := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@example.invalid"}
	g := &gitx.Git{Dir: root, Env: gitx.Env(environ, false)}
	git := func(args ...string) {
		t.Helper()
		if _, err := g.Run(0, args...); err != nil {
			t.Fatal(err)
		}
	}
	write("memex.toml", "format = 1\n")
	write("rules/2026-01-10-target.md", entryText("Target", "")+"[[2026-01-10-missing]]\n")
	write("rules/2026-01-10-other.md", "broken")
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")

	run := func(env []string) (string, int) {
		var out, errb bytes.Buffer
		e := &Env{Stdout: &out, Stderr: &errb, Environ: env, Now: time.Now, Store: root, StoreSet: true,
			Getenv: func(k string) string {
				for _, kv := range env {
					if len(kv) > len(k) && kv[:len(k)+1] == k+"=" {
						return kv[len(k)+1:]
					}
				}
				return ""
			}}
		code := Check(e, CheckOptions{Staged: true, Limit: DefaultCheckLimit})
		return out.String() + errb.String(), code
	}
	if out, code := run(environ); code != 0 || out != "check: 0 errors, 0 warnings\n" {
		t.Errorf("nothing staged: exit %d\n%s", code, out)
	}
	// A staged entry linking to the target: both are checked, from the index; the
	// unrelated broken entry is not.
	write("rules/2026-01-10-new.md", entryText("New", "[[2026-01-10-target]]\n"))
	git("add", "rules/2026-01-10-new.md")
	write("rules/2026-01-10-new.md", "unstaged edit")
	want := "rules/2026-01-10-target.md: E-LINK line 10: [[2026-01-10-missing]] is not the id of an entry\n" +
		"check: 1 errors, 0 warnings\n"
	if out, code := run(environ); code != 1 || out != want {
		t.Errorf("staged: exit %d\n%s\nwant\n%s", code, out, want)
	}
	// An alternative index, as `git commit -- <paths>` uses: GIT_INDEX_FILE is kept.
	alt := filepath.Join(root, ".git", "alt-index")
	altEnv := append(environ, "GIT_INDEX_FILE="+alt)
	ga := &gitx.Git{Dir: root, Env: gitx.Env(altEnv, true)}
	if _, err := ga.Run(0, "read-tree", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if out, code := run(altEnv); code != 0 || out != "check: 0 errors, 0 warnings\n" {
		t.Errorf("GIT_INDEX_FILE: exit %d\n%s", code, out)
	}
}
