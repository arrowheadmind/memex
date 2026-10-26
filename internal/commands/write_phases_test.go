package commands

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/write"
)

// lockedWriteEnv returns a committed store whose write lock is held independently of the
// command under test. It exercises the phase boundary in SPEC §9.1 without the conformance
// runner having to coordinate two memex processes.
func lockedWriteEnv(t *testing.T, stdin io.Reader) (*Env, func()) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "store")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memex.toml"), []byte("format = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, typ := range spec.Types {
		if err := os.MkdirAll(filepath.Join(root, typ.Directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(config, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[user]\n\tname = test\n\temail = test@example.invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitT(t, root, "init", "-q")
	gitT(t, root, "add", "-A")
	gitT(t, root, "commit", "-q", "-m", "seed")
	held, err := write.Begin(root, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	oldWait := write.LockWait
	write.LockWait = 200 * time.Millisecond
	vars := map[string]string{
		"HOME": dir, "MEMEX_CONFIG": config, "MEMEX_TODAY": "2026-01-15",
	}
	var out, errb bytes.Buffer
	e := &Env{
		Stdin: stdin, Stdout: &out, Stderr: &errb, Store: root, StoreSet: true,
		Getenv: func(k string) string { return vars[k] }, Now: time.Now,
	}
	return e, func() {
		write.LockWait = oldWait
		held.End()
	}
}

func assertLockRefusal(t *testing.T, e *Env, run func() (int, *UsageError)) {
	t.Helper()
	code, uerr := run()
	if code != spec.ExitLock || uerr != nil {
		t.Fatalf("exit %d, usage error %v; want exit %d and no usage error", code, uerr, spec.ExitLock)
	}
	if errb := e.Stderr.(*bytes.Buffer).String(); !strings.HasPrefix(errb, "memex: the store is locked by another memex;") {
		t.Fatalf("stderr %q does not report the held lock", errb)
	}
}

func TestInputsAreReadAfterLock(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	rule := spec.LookupType("rule")
	journal := spec.LookupType("journal")
	tests := []struct {
		name string
		run  func(*Env) (int, *UsageError)
	}{
		{"new body", func(e *Env) (int, *UsageError) {
			return New(e, NewOptions{Type: rule, Title: "T", Summary: "S", Body: missing})
		}},
		{"capture body", func(e *Env) (int, *UsageError) {
			return New(e, NewOptions{Type: journal, Title: "T", Summary: "S", Body: missing})
		}},
		{"promote body", func(e *Env) (int, *UsageError) {
			return Promote(e, PromoteOptions{Source: "missing", New: NewOptions{Type: rule, Title: "T", Summary: "S", Body: missing}})
		}},
		{"capture from", func(e *Env) (int, *UsageError) {
			return New(e, NewOptions{Type: journal, From: missing})
		}},
		{"set batch file", func(e *Env) (int, *UsageError) { return SetBatch(e, missing) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, release := lockedWriteEnv(t, strings.NewReader(""))
			defer release()
			assertLockRefusal(t, e, func() (int, *UsageError) { return tt.run(e) })
		})
	}
}

func TestStandardInputIsUnreadWhileLockIsHeld(t *testing.T) {
	for _, name := range []string{"new", "set batch"} {
		t.Run(name, func(t *testing.T) {
			r, w := io.Pipe()
			defer r.Close()
			defer w.Close()
			e, release := lockedWriteEnv(t, r)
			defer release()
			run := func() (int, *UsageError) {
				if name == "new" {
					return New(e, NewOptions{Type: spec.LookupType("rule"), Title: "T", Summary: "S", Body: "-"})
				}
				return SetBatch(e, "-")
			}
			type result struct {
				code int
				err  *UsageError
			}
			done := make(chan result, 1)
			go func() {
				code, err := run()
				done <- result{code, err}
			}()
			select {
			case got := <-done:
				if got.code != spec.ExitLock || got.err != nil {
					t.Fatalf("exit %d, usage error %v; want exit %d and no usage error", got.code, got.err, spec.ExitLock)
				}
				if errb := e.Stderr.(*bytes.Buffer).String(); !strings.HasPrefix(errb, "memex: the store is locked by another memex;") {
					t.Fatalf("stderr %q does not report the held lock", errb)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("command read standard input before the held lock refused it")
			}
		})
	}
}
