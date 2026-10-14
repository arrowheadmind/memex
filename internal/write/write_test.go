package write

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// repo makes a store repository with a bare origin, in a temporary directory, with a
// private git identity. It returns the store and the origin.
func repo(t *testing.T) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gc := filepath.Join(dir, "gitconfig")
	os.WriteFile(gc, []byte("[user]\n\tname = test\n\temail = test@example.invalid\n[init]\n\tdefaultBranch = main\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", gc)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	st, origin := filepath.Join(dir, "store"), filepath.Join(dir, "origin.git")
	os.MkdirAll(filepath.Join(st, "rules"), 0o755)
	os.WriteFile(filepath.Join(st, "memex.toml"), []byte("format = 1\n"), 0o644)
	os.WriteFile(filepath.Join(st, ".gitignore"), []byte(".memex/\n"), 0o644)
	gitIn(t, st, "init", "-q")
	gitIn(t, st, "add", "-A")
	gitIn(t, st, "commit", "-q", "-m", "seed")
	gitIn(t, dir, "clone", "-q", "--bare", st, origin)
	gitIn(t, st, "remote", "add", "origin", origin)
	gitIn(t, st, "fetch", "-q", "origin")
	gitIn(t, st, "branch", "-q", "--set-upstream-to=origin/main")
	return st, origin
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

const entry = "+++\ntitle = \"A rule\"\nstatus = \"active\"\ncreated = 2026-01-15\nsummary = \"S.\"\ntags = [\"x\"]\n+++\n\n" +
	"This body is long enough to pass the content measure of eighty code points, with room to spare.\n"

var today = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

func TestLockTimeout(t *testing.T) {
	st, _ := repo(t)
	held, err := lock(st, time.Second, func() bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = Begin(st, 20, 300*time.Millisecond)
	var le *LockError
	if !errors.As(err, &le) {
		t.Fatalf("Begin with the lock held: %v, want a LockError", err)
	}
	if d := time.Since(start); d < 300*time.Millisecond {
		t.Errorf("gave up after %v, before the wait", d)
	}
	held.Close()
	s, err := Begin(st, 20, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("Begin after release: %v", err)
	}
	s.End()
}

func TestLockIsFlock(t *testing.T) {
	st, _ := repo(t)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	f, err := os.OpenFile(filepath.Join(st, LockFile), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Errorf("flock while a session holds the lock: %v, want EWOULDBLOCK", err)
	}
}

func TestIndexLockRetry(t *testing.T) {
	st, _ := repo(t)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	lockPath := filepath.Join(st, ".git", "index.lock")
	os.WriteFile(lockPath, nil, 0o644)
	var slept []time.Duration
	s.sleep = func(d time.Duration) {
		slept = append(slept, d)
		if len(slept) == 3 {
			os.Remove(lockPath)
		}
	}
	files := []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	o, err := s.Commit(Paths(files), "memex: new rule 2026-01-15-a-rule")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !strings.HasPrefix(o.Line, "pushed ") || len(slept) != 3 {
		t.Errorf("outcome %q after %v of retries", o.Line, slept)
	}
}

func TestIndexLockGivesUp(t *testing.T) {
	st, _ := repo(t)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	os.WriteFile(filepath.Join(st, ".git", "index.lock"), nil, 0o644)
	var total time.Duration
	s.sleep = func(d time.Duration) { total += d }
	files := []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	_, err = s.Commit(Paths(files), "msg")
	var ie *IndexLockError
	if !errors.As(err, &ie) {
		t.Fatalf("Commit with a stuck index.lock: %v, want an IndexLockError", err)
	}
	if total != 6200*time.Millisecond {
		t.Errorf("retried for %v, want 6.2s", total)
	}
	if _, err := os.Stat(filepath.Join(st, files[0].Path)); !os.IsNotExist(err) {
		t.Errorf("the new file was not removed: %v", err)
	}
}

// TestPostRebaseSHA checks that the outcome names the commit as it exists on the remote.
func TestPostRebaseSHA(t *testing.T) {
	st, origin := repo(t)
	other := filepath.Join(filepath.Dir(st), "other")
	gitIn(t, filepath.Dir(st), "clone", "-q", origin, other)
	gitIn(t, other, "commit", "-q", "--allow-empty", "-m", "remote")
	gitIn(t, other, "push", "-q", "origin", "main")
	gitIn(t, st, "commit", "-q", "--allow-empty", "-m", "local") // diverged: no fast-forward
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	s.Freshen()
	files := []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	before := gitIn(t, st, "rev-parse", "HEAD")
	o, err := s.Commit(Paths(files), "memex: new rule 2026-01-15-a-rule")
	if err != nil {
		t.Fatal(err)
	}
	remote := gitIn(t, origin, "rev-parse", "main")
	if o.Line != "pushed "+remote[:7] || strings.HasPrefix(before, remote[:7]) {
		t.Errorf("outcome %q; remote main is %s", o.Line, remote)
	}
}

func TestValidationRestores(t *testing.T) {
	st, _ := repo(t)
	old := filepath.Join(st, "rules", "2026-01-15-a-rule.md")
	os.WriteFile(old, []byte(entry), 0o644)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	bad := strings.Replace(entry, "status = \"active\"", "status = \"bogus\"", 1)
	err = s.Write(today, []File{
		{Path: "rules/2026-01-15-a-rule.md", Data: []byte(bad)},
		{Path: "rules/2026-01-15-b.md", Data: []byte(entry)},
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Write of an invalid entry: %v", err)
	}
	if got, _ := os.ReadFile(old); string(got) != entry {
		t.Errorf("old entry not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(st, "rules", "2026-01-15-b.md")); !os.IsNotExist(err) {
		t.Errorf("new entry not removed: %v", err)
	}
}

// TestRebaseInProgress stops a real rebase (an --exec that fails) and checks that writes
// refuse before anything is written and that sync reports it (SPEC §9.1 step 2, §9.5).
func TestRebaseInProgress(t *testing.T) {
	st, _ := repo(t)
	os.WriteFile(filepath.Join(st, "README.md"), []byte("x\n"), 0o644)
	gitIn(t, st, "add", "README.md")
	gitIn(t, st, "commit", "-q", "-m", "one")
	cmd := exec.Command("git", "rebase", "-q", "--exec", "exit 1", "HEAD~1")
	cmd.Dir = st
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("the rebase did not stop:\n%s", out)
	}
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	var ie *InProgressError
	if err := s.CheckInProgress(); !errors.As(err, &ie) || ie.Op != "rebase" {
		t.Errorf("CheckInProgress: %v", err)
	}
	if state, code := s.Sync(); state != "NOT SYNCED: rebase in progress" || code != 5 {
		t.Errorf("sync: %q, exit %d", state, code)
	}
}

// slowGit puts a git first on PATH that sleeps instead of running the named subcommand.
func slowGit(t *testing.T, sub string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a; do [ \"$a\" = " + sub + " ] && exec sleep 30; done\nexec " + real + " \"$@\"\n"
	os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// interruptAfter sends SIGTERM to this process after d; the session's handler catches it.
func interruptAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
}

func TestInterruptBeforeCommit(t *testing.T) {
	st, _ := repo(t)
	slowGit(t, "commit")
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	files := []File{{Path: "tasks/2026-01-15-a-rule.md", Data: []byte(strings.Replace(entry, "active", "open", 1))}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	before := gitIn(t, st, "rev-parse", "HEAD")
	interruptAfter(500 * time.Millisecond)
	start := time.Now()
	_, err = s.Commit(Paths(files), "msg")
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Commit: %v, want ErrInterrupted", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v: the slow git was not stopped", d)
	}
	if _, err := os.Stat(filepath.Join(st, "tasks")); !os.IsNotExist(err) {
		t.Errorf("tasks/ was not removed: %v", err)
	}
	if gitIn(t, st, "rev-parse", "HEAD") != before || gitIn(t, st, "status", "--porcelain") != "" {
		t.Errorf("store changed: %s", gitIn(t, st, "status", "--porcelain"))
	}
}

func TestInterruptAfterCommit(t *testing.T) {
	st, _ := repo(t)
	slowGit(t, "pull")
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	files := []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	interruptAfter(500 * time.Millisecond)
	o, err := s.Commit(Paths(files), "msg")
	if err != nil {
		t.Fatal(err)
	}
	head := gitIn(t, st, "rev-parse", "HEAD")
	if o.Line != "committed "+head[:7]+", NOT pushed: interrupted" || o.Code != 5 {
		t.Errorf("outcome %q, exit %d", o.Line, o.Code)
	}
}

func TestWriteErrorIsRelative(t *testing.T) {
	st, _ := repo(t)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	os.Chmod(filepath.Join(st, "rules"), 0o555)
	defer os.Chmod(filepath.Join(st, "rules"), 0o755)
	err = s.Write(today, []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}})
	var we *WriteError
	if !errors.As(err, &we) || we.Error() != "cannot write rules/2026-01-15-a-rule.md: permission denied; nothing was changed" {
		t.Errorf("Write into a read-only directory: %v", err)
	}
}

func TestChangedWhileWorking(t *testing.T) {
	st, _ := repo(t)
	p := filepath.Join(st, "rules", "2026-01-15-a-rule.md")
	os.WriteFile(p, []byte(entry), 0o644)
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	os.WriteFile(p, []byte(entry+"edited\n"), 0o644)
	err = s.Write(today, []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte("new"), Check: true, Expect: []byte(entry)}})
	var ce *ChangedError
	if !errors.As(err, &ce) {
		t.Errorf("Write over a changed file: %v", err)
	}
	if got, _ := os.ReadFile(p); string(got) != entry+"edited\n" {
		t.Errorf("the edit was overwritten: %q", got)
	}
}

// TestStagedDiffers changes a file between memex's read-back and git add, with a git whose
// add appends a line first: the commit is refused and the store restored (SPEC §9.1 step 6).
func TestStagedDiffers(t *testing.T) {
	st, _ := repo(t)
	real, _ := exec.LookPath("git")
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = add ]; then for a; do [ -f \"$a\" ] && echo tampered >> \"$a\"; done; fi\nexec " + real + " \"$@\"\n"
	os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	s, err := Begin(st, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.End()
	files := []File{{Path: "rules/2026-01-15-a-rule.md", Data: []byte(entry)}}
	if err := s.Write(today, files); err != nil {
		t.Fatal(err)
	}
	_, err = s.Commit(Paths(files), "msg")
	var ce *ChangedError
	if !errors.As(err, &ce) || ce.Error() != "rules/2026-01-15-a-rule.md changed while memex was working; retry" {
		t.Fatalf("Commit after a changed add: %v", err)
	}
	if out := gitIn(t, st, "status", "--porcelain"); out != "" {
		t.Errorf("store not restored: %q", out)
	}
}
