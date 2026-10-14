package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/spec"
)

// AgentMD is the AGENT.md that init writes (SPEC Appendix A).
const AgentMD = "# Agent instructions\n" +
	"\n" +
	"This store is the shared memory for this operator's work. It is managed with `memex`.\n" +
	"\n" +
	"- If you are new to memex: `memex help agents` (short), or `memex help --all` (everything).\n" +
	"- Before any non-trivial task: `memex preflight \"<the task, in content words>\"` (skip a, the, for).\n" +
	"  Read the `matched k/n` label and any `weak results` line before trusting a result.\n" +
	"- After any non-trivial task:\n" +
	"  `memex capture --title=\"<task>\" --summary=\"<one sentence>\" --body - <<'EOF' … EOF`.\n" +
	"- Never write knowledge into a coding tool's own memory or rules files.\n" +
	"  Knowledge lives here only.\n" +
	"- Edit entries directly when needed, then run `memex save`.\n" +
	"\n" +
	"Operator-specific instructions follow.\n"

// PreCommit is the .githooks/pre-commit that init writes (SPEC Appendix A).
const PreCommit = "#!/bin/sh\n" +
	"# Written by memex init. Add your own checks as executables in .githooks/pre-commit.d/.\n" +
	"store=$(git rev-parse --show-toplevel) || exit 1\n" +
	"memex --store \"$store\" check --staged || exit 1\n" +
	"if [ -d \"$store/.githooks/pre-commit.d\" ]; then\n" +
	"  for hook in \"$store\"/.githooks/pre-commit.d/*; do\n" +
	"    [ -x \"$hook\" ] && { \"$hook\" || exit 1; }\n" +
	"  done\n" +
	"fi\n" +
	"exit 0\n"

// GitignoreLine is the line .gitignore must contain (SPEC §4).
const GitignoreLine = ".memex/"

// CreateLayout creates a store directory as `memex init PATH` does (SPEC §11.1): `git init`
// when it is not a repository, the manifest, AGENT.md, the type directories, files/,
// .gitignore and the pre-commit hook, and core.hooksPath. Existing nonempty files are left
// alone and missing lines are appended to .gitignore. It does not commit. It returns one line
// per item it created or changed, in that order.
func CreateLayout(dir, name string, g *gitx.Git) ([]string, error) {
	var done []string
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if _, err := g.Run("init", "-q"); err != nil {
			return nil, err
		}
		done = append(done, "created .git/")
	}
	writeIfAbsent := func(rel, content string, mode os.FileMode) error {
		p := filepath.Join(dir, rel)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			return err
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
		done = append(done, "created "+filepath.ToSlash(rel))
		return nil
	}
	mkdir := func(rel string) error {
		p := filepath.Join(dir, rel)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return nil
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
		done = append(done, "created "+rel+"/")
		return nil
	}
	manifest := fmt.Sprintf("format = %d\nname = %s\n", spec.StoreFormat, entry.Quote(name))
	if err := writeIfAbsent(ManifestFile, manifest, 0o644); err != nil {
		return nil, err
	}
	if err := writeIfAbsent("AGENT.md", AgentMD, 0o644); err != nil {
		return nil, err
	}
	for _, t := range spec.Types {
		if err := mkdir(t.Directory); err != nil {
			return nil, err
		}
	}
	if err := mkdir("files"); err != nil {
		return nil, err
	}
	gi := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gi)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if !containsLine(string(data), GitignoreLine) {
		s := string(data)
		if s != "" && s[len(s)-1] != '\n' {
			s += "\n"
		}
		if err := os.WriteFile(gi, []byte(s+GitignoreLine+"\n"), 0o644); err != nil {
			return nil, err
		}
		if len(data) == 0 {
			done = append(done, "created .gitignore")
		} else {
			done = append(done, "updated .gitignore")
		}
	}
	if err := writeIfAbsent(filepath.Join(".githooks", "pre-commit"), PreCommit, 0o755); err != nil {
		return nil, err
	}
	if out, _ := g.Run("config", "core.hooksPath"); out != ".githooks\n" {
		if _, err := g.Run("config", "core.hooksPath", ".githooks"); err != nil {
			return nil, err
		}
		done = append(done, "set core.hooksPath .githooks")
	}
	return done, nil
}

func containsLine(text, line string) bool {
	for _, l := range splitLinesSimple(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLinesSimple(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
