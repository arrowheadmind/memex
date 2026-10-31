package neutral

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestToolMemory(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	ManagedDir = filepath.Join(root, "etc")
	defer func() { ManagedDir = "/etc/claude-code" }()
	noEnv := func(string) string { return "" }

	if ok, msg := ToolMemory(noEnv, home); !ok || msg != "~/.claude does not exist" {
		t.Errorf("no ~/.claude: %v %q", ok, msg)
	}
	write(t, filepath.Join(home, ".claude", "settings.json"), `{"autoMemoryEnabled": true}`)
	if ok, msg := ToolMemory(noEnv, home); ok || !strings.Contains(msg, "not disabled") {
		t.Errorf("enabled: %v %q", ok, msg)
	}
	write(t, filepath.Join(ManagedDir, "managed-settings.d", "10-x.json"), `not json`)
	write(t, filepath.Join(ManagedDir, "managed-settings.d", "20-y.json"), `{"hooks":{},"autoMemoryEnabled":false}`)
	target := filepath.Join(ManagedDir, "managed-settings.d", "20-y.json")
	if ok, msg := ToolMemory(noEnv, home); !ok || msg != "Claude Code auto memory is disabled in "+target {
		t.Errorf("managed drop-in: %v %q", ok, msg)
	}
	// A settings file installed as a symbolic link is read through it.
	os.Remove(target)
	real := filepath.Join(root, "static.json")
	write(t, real, `{"autoMemoryEnabled":false}`)
	if err := os.Symlink(real, target); err != nil {
		t.Fatal(err)
	}
	if ok, _ := ToolMemory(noEnv, home); !ok {
		t.Error("a symbolic link to a disabling file should count")
	}
	write(t, filepath.Join(ManagedDir, "managed-settings.json"), `{"autoMemoryEnabled":false}`)
	if ok, msg := ToolMemory(noEnv, home); !ok || msg != "Claude Code auto memory is disabled in "+filepath.Join(ManagedDir, "managed-settings.json") {
		t.Errorf("managed file first: %v %q", ok, msg)
	}
	env := func(k string) string {
		if k == "CLAUDE_CODE_DISABLE_AUTO_MEMORY" {
			return "1"
		}
		return ""
	}
	if ok, msg := ToolMemory(env, home); !ok || !strings.Contains(msg, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1") {
		t.Errorf("env: %v %q", ok, msg)
	}
}

func TestIsPointer(t *testing.T) {
	line := "Read ~/memex-store/AGENT.md before starting work, and follow it.\n"
	if !IsPointer(line, "/home/x/memex-store", "/home/x") {
		t.Error("~ form should match the absolute store")
	}
	if IsPointer(strings.TrimSuffix(line, "\n"), "/home/x/memex-store", "/home/x") {
		t.Error("the line feed is part of the pointer")
	}
	if IsPointer(line, "", "/home/x") {
		t.Error("no store, no pointer")
	}
}

func TestScanDepthAndExclusions(t *testing.T) {
	ws := t.TempDir()
	for _, d := range []string{"a/b/c/d", "a/b/c/d/e", "node_modules/x", "vendor/y", ".worktrees/z", "r/.git"} {
		if err := os.MkdirAll(filepath.Join(ws, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := Scan{Workspace: ws}
	got := map[string]bool{}
	for _, d := range s.scanDirs() {
		rel, _ := filepath.Rel(ws, d)
		got[rel] = true
	}
	for _, want := range []string{".", "a/b/c/d", "r"} {
		if !got[want] {
			t.Errorf("%s not visited", want)
		}
	}
	for _, no := range []string{"a/b/c/d/e", "node_modules", "vendor", ".worktrees", "r/.git"} {
		if got[no] {
			t.Errorf("%s visited", no)
		}
	}
	fs := s.Pointers()
	if len(fs) != 1 || fs[0].Code != "W-NO-POINTER" || fs[0].Path != filepath.Join(ws, "r") {
		t.Errorf("findings %v", fs)
	}
}

func TestRepositories(t *testing.T) {
	ws := t.TempDir()
	store := filepath.Join(ws, "store")
	for _, d := range []string{"b/.git", "a/.git", "group/a/.git", "group/c/.git", "store/.git",
		"node_modules/x/.git", "d1/d2/d3/d4/d5/.git"} {
		if err := os.MkdirAll(filepath.Join(ws, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(ws, "wt", ".git"), "gitdir: elsewhere\n") // a worktree's .git file
	got := strings.Join(Scan{Workspace: ws, Store: store}.Repositories(), ",")
	if want := "a,b,c,wt"; got != want {
		t.Errorf("Repositories = %q, want %q", got, want)
	}
	if got := (Scan{}).Repositories(); got != nil {
		t.Errorf("no workspace: %q", got)
	}
}
