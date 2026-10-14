package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/arrowheadmind/memex/internal/write"
)

// TestPreflightLockHeld checks that a write lock held by another memex makes preflight's
// sync a NOT SYNCED line, without changing the exit code (SPEC §11.4).
func TestPreflightLockHeld(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "store")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "memex.toml"), []byte("format = 1\n"), 0o644)
	cfg := filepath.Join(dir, "config.toml")
	os.WriteFile(cfg, nil, 0o644)
	gitT(t, root, "init", "-q")
	held, err := write.Begin(root, 20, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.End()
	saved := write.PreflightLockWait
	write.PreflightLockWait = 200 * time.Millisecond
	defer func() { write.PreflightLockWait = saved }()

	vars := map[string]string{"MEMEX_STORE": root, "MEMEX_CONFIG": cfg, "MEMEX_TODAY": "2026-01-15"}
	var out, errb bytes.Buffer
	e := &Env{Stdout: &out, Stderr: &errb, Getenv: func(k string) string { return vars[k] }, Now: time.Now}
	code, uerr := Preflight(e, PreflightOptions{MaxChars: DefaultMaxChars, Limit: -1})
	if uerr != nil || code != 0 {
		t.Fatalf("exit %d, %v; stderr %s", code, uerr, errb.String())
	}
	if want := "store: NOT SYNCED: locked by another memex for 0s\n\n## Always\n"; out.String() != want {
		t.Errorf("output %q, want %q", out.String(), want)
	}
}
