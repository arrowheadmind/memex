// Package neutral implements SPEC §12, tool neutrality: the pointer check of
// `check --workspace`, the leak scan of `check --workspace` and `doctor`, and doctor's
// tool-memory check.
package neutral

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/store"
)

// MaxDepth is how far below the workspace the scan reaches: directories of depth 0 (the
// workspace itself) to MaxDepth are scanned (SPEC §12 rule 2).
const MaxDepth = 4

// excluded are the directory names the scan never enters (SPEC §12 rule 2).
var excluded = []string{".git", "node_modules", "vendor", ".worktrees"}

// PointerNames are the files the pointer check compares with the pointer line, relative to
// each scanned directory (SPEC §12 rule 2).
var PointerNames = []string{"CLAUDE.md", "CLAUDE.local.md", ".claude/CLAUDE.md", "AGENTS.md", "GEMINI.md"}

// initNames are the pointer files `memex init --pointers` writes (SPEC §11.1).
var initNames = []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"}

// workspaceLeaks are the leak locations relative to each scanned directory; a trailing "/"
// means every file below that directory, at any depth (SPEC §12 rule 3).
var workspaceLeaks = []string{".cursorrules", ".cursor/rules/", ".claude/rules/", ".github/copilot-instructions.md", ".windsurfrules"}

// homeFiles are the single-file leak locations under the home directory. A file there that
// holds exactly the pointer line is the tool's global pointer, not a leak (SPEC §12 rule 3).
var homeFiles = []string{".claude/CLAUDE.md", ".codex/AGENTS.md", ".gemini/GEMINI.md"}

// pointerPattern is the pointer line of SPEC §12 rule 2, with the store path as its group.
var pointerPattern = regexp.MustCompile(`^Read (.+)/AGENT\.md before starting work, and follow it\.\n$`)

// PointerText is the pointer line naming store (SPEC §12 rule 2).
func PointerText(store string) string {
	return fmt.Sprintf("Read %s/AGENT.md before starting work, and follow it.\n", store)
}

// IsPointer reports whether content is the pointer line naming root, comparing the paths
// after the §3.1 expansion of both (SPEC §12 rule 2).
func IsPointer(content, root, home string) bool {
	m := pointerPattern.FindStringSubmatch(content)
	if m == nil || root == "" {
		return false
	}
	got, err1 := config.Expand(m[1], home)
	want, err2 := config.Expand(root, home)
	return err1 == nil && err2 == nil && got == want
}

// Scan is one run of the §12 scans.
type Scan struct {
	Home      string // "" when HOME is unset: the home locations are skipped
	Workspace string // absolute, expanded; "" when none is configured
	Store     string // the resolved store root; "" when none resolved
}

// scanDirs lists the directories the workspace scan visits: the workspace and every
// directory below it to MaxDepth, without entering excluded names, symbolic links or the
// store itself, in path order.
func (s Scan) scanDirs() []string {
	if s.Workspace == "" {
		return nil
	}
	var storeInfo fs.FileInfo
	if s.Store != "" {
		storeInfo, _ = os.Stat(s.Store)
	}
	var dirs []string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		dirs = append(dirs, dir)
		if depth == MaxDepth {
			return
		}
		items, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, it := range items {
			if !it.IsDir() || slices.Contains(excluded, it.Name()) { // a symlink is not IsDir
				continue
			}
			p := filepath.Join(dir, it.Name())
			if storeInfo != nil {
				if info, err := os.Lstat(p); err == nil && os.SameFile(info, storeInfo) {
					continue
				}
			}
			walk(p, depth+1)
		}
	}
	if info, err := os.Stat(s.Workspace); err == nil && info.IsDir() {
		if storeInfo == nil || !os.SameFile(info, storeInfo) {
			walk(filepath.Clean(s.Workspace), 0)
		}
	}
	return dirs
}

// Pointers returns the E-POINTER and W-NO-POINTER findings of the workspace (SPEC §12
// rule 2), with absolute paths.
func (s Scan) Pointers() []store.Finding {
	var out []store.Finding
	seen := map[string]bool{} // a/.claude/CLAUDE.md is checked as a's, not again as .claude's
	for _, dir := range s.scanDirs() {
		for _, name := range PointerNames {
			p := filepath.Join(dir, filepath.FromSlash(name))
			if seen[p] {
				continue
			}
			seen[p] = true
			info, err := os.Lstat(p)
			if err != nil {
				continue
			}
			viaInit := slices.Contains(initNames, name)
			switch {
			case info.Mode()&fs.ModeSymlink != 0:
				msg := "is a symbolic link, not the pointer file; delete it"
				if viaInit {
					msg += ", then: memex init --pointers " + dir
				}
				out = append(out, store.Finding{Path: p, Code: "E-POINTER", Message: msg})
			case !info.Mode().IsRegular():
				out = append(out, store.Finding{Path: p, Code: "E-POINTER", Message: "is not a regular file; delete it"})
			default:
				data, err := os.ReadFile(p)
				if err == nil && IsPointer(string(data), s.Store, s.Home) {
					continue
				}
				msg := "is not the pointer to this store; move any knowledge with memex capture --from " + p + ", then "
				if viaInit {
					msg += "memex init --pointers " + dir + " --force"
				} else {
					msg += "delete the file"
				}
				out = append(out, store.Finding{Path: p, Code: "E-POINTER", Message: msg})
			}
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		_, e1 := os.Lstat(filepath.Join(dir, "CLAUDE.md"))
		_, e2 := os.Lstat(filepath.Join(dir, "AGENTS.md"))
		if e1 != nil && e2 != nil {
			out = append(out, store.Finding{Path: dir, Code: "W-NO-POINTER",
				Message: "repository has no CLAUDE.md or AGENTS.md; run memex init --pointers " + dir})
		}
	}
	return out
}

// Leaks returns the leak scan of SPEC §12 rule 3: every regular non-empty file at the
// listed locations, as absolute paths in path order, each once.
func (s Scan) Leaks() []string {
	seen := map[string]bool{}
	add := func(p string) {
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return
		}
		seen[p] = true
	}
	if s.Home != "" {
		projects := filepath.Join(s.Home, ".claude", "projects")
		if items, err := os.ReadDir(projects); err == nil {
			for _, it := range items {
				if it.IsDir() {
					s.walkFiles(filepath.Join(projects, it.Name(), "memory"), add)
				}
			}
		}
		for _, name := range homeFiles {
			p := filepath.Join(s.Home, filepath.FromSlash(name))
			if data, err := os.ReadFile(p); err == nil && IsPointer(string(data), s.Store, s.Home) {
				continue
			}
			add(p)
		}
	}
	for _, dir := range s.scanDirs() {
		for _, name := range workspaceLeaks {
			p := filepath.Join(dir, filepath.FromSlash(strings.TrimSuffix(name, "/")))
			if strings.HasSuffix(name, "/") {
				s.walkFiles(p, add)
			} else {
				add(p)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// walkFiles calls add for every file below dir, at any depth, without following symbolic
// links or entering excluded names.
func (s Scan) walkFiles(dir string, add func(string)) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && slices.Contains(excluded, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		add(p)
		return nil
	})
}

// LeakFindings are the W-LEAK findings of check --workspace for the leak scan.
func LeakFindings(paths []string) []store.Finding {
	out := make([]store.Finding, len(paths))
	for i, p := range paths {
		out[i] = store.Finding{Path: p, Code: "W-LEAK",
			Message: "knowledge in a coding tool's own memory; move it into the store: memex capture --from " + p + ", then delete the file"}
	}
	return out
}

// ManagedDir is where Claude Code's managed settings live on a POSIX system (SPEC §12
// rule 5). It is a variable only so that tests can point it elsewhere.
var ManagedDir = "/etc/claude-code"

// ToolMemory is doctor's tool-memory check (SPEC §12 rule 5): whether Claude Code's auto
// memory is disabled, and the message that says how it was decided.
func ToolMemory(getenv func(string) string, home string) (ok bool, msg string) {
	if home == "" {
		return true, "HOME is not set; no ~/.claude to check"
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err != nil {
		return true, "~/.claude does not exist"
	}
	if getenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY") == "1" {
		return true, "Claude Code auto memory is disabled by CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"
	}
	files := []struct{ path, shown string }{{filepath.Join(home, ".claude", "settings.json"), "~/.claude/settings.json"}}
	managed := filepath.Join(ManagedDir, "managed-settings.json")
	files = append(files, struct{ path, shown string }{managed, managed})
	if matches, err := filepath.Glob(filepath.Join(ManagedDir, "managed-settings.d", "*.json")); err == nil {
		slices.Sort(matches)
		for _, m := range matches {
			files = append(files, struct{ path, shown string }{m, m})
		}
	}
	for _, f := range files {
		if autoMemoryOff(f.path) {
			return true, "Claude Code auto memory is disabled in " + f.shown
		}
	}
	return false, `Claude Code auto memory is not disabled; set "autoMemoryEnabled": false in ~/.claude/settings.json`
}

// autoMemoryOff reports whether the JSON object in the file at p has the top-level member
// "autoMemoryEnabled": false. Settings files are read through symbolic links.
func autoMemoryOff(p string) bool {
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return false
	}
	v, ok := obj["autoMemoryEnabled"]
	return ok && strings.TrimSpace(string(v)) == "false"
}
