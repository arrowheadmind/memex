package store

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/arrowheadmind/memex/internal/gitx"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, data := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			os.MkdirAll(full, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestReadManifest(t *testing.T) {
	cases := map[string]string{
		"format = 1\nname = \"x\"\n":           "",
		"format = 1\n":                         "",
		"format = 1\nnmae = \"x\"\n":           `E-MANIFEST unknown key "nmae" (did you mean "name"?)`,
		"name = \"x\"\n":                       `E-MANIFEST missing key "format"`,
		"format = \"1\"\nname = 2\n":           "E-MANIFEST format must be an integer;E-MANIFEST name must be a string",
		"format = 1\nname = \"x\"\nname = 1\n": "E-MANIFEST invalid TOML at line 3",
	}
	for text, want := range cases {
		m, fs, err := ReadManifest(WorkTree{writeTree(t, map[string]string{"memex.toml": text})})
		var got []string
		for _, f := range fs {
			got = append(got, f.Code+" "+f.Message)
		}
		if err != nil || strings.Join(got, ";") != want || m == nil {
			t.Errorf("%q: %v, %q, want %q", text, err, got, want)
		}
	}
	for text, want := range map[string]string{
		"format = 2\n": "store format 2 is newer than this memex reads (1); upgrade memex",
		"format = 0\n": "store format 0 is older than this memex reads (1)",
	} {
		_, _, err := ReadManifest(WorkTree{writeTree(t, map[string]string{"memex.toml": text})})
		var fe *FormatError
		if !errors.As(err, &fe) || !strings.HasPrefix(fe.Msg, want) {
			t.Errorf("%q: %v", text, err)
		}
	}
}

func layoutString(l *Layout) string {
	var b strings.Builder
	for _, f := range l.Files {
		fmt.Fprintf(&b, "entry %s %s %s\n", f.Path, f.Type.Name, f.ID)
	}
	fmt.Fprintf(&b, "owners %v\n", l.Owners)
	for _, f := range l.Findings {
		fmt.Fprintf(&b, "%s %s\n", f.Path, f.Code)
	}
	return b.String()
}

var layoutFiles = map[string]string{
	"memex.toml": "format = 1\n", "AGENT.md": "", "README.md": "", "LICENSE": "", "flake.nix": "",
	"flake.lock": "", "import-report.json": "", ".gitignore": ".memex/\n", ".githooks/pre-commit": "",
	"rules/2026-01-15-a.md": "", "rules/notes.txt": "", "rules/sub/x.md": "",
	"journal/2026-01-15-j.md": "", "files/2026-01-15-a/x.png": "", "files/loose.png": "",
	"notes.md": "", "scratch/x": "",
}

const layoutWant = `entry journal/2026-01-15-j.md journal 2026-01-15-j
entry rules/2026-01-15-a.md rule 2026-01-15-a
owners [2026-01-15-a]
files/loose.png E-FILES-OWNER
notes.md E-NOT-ENTRY
rules/notes.txt E-NOT-ENTRY
rules/sub E-NOT-ENTRY
scratch E-NOT-ENTRY
`

func TestEnumerate(t *testing.T) {
	root := writeTree(t, layoutFiles)
	os.MkdirAll(filepath.Join(root, ".memex"), 0o755)
	l, err := Enumerate(WorkTree{root})
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutString(l); got != layoutWant {
		t.Errorf("Enumerate:\n%s\nwant\n%s", got, layoutWant)
	}
}

func TestReadIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := writeTree(t, layoutFiles)
	home := t.TempDir()
	setGitEnv(t, home)
	g := gitx.New(root, 0)
	run := func(args ...string) {
		t.Helper()
		if _, err := g.Run(args...); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	run("add", "-A")
	idx, changed, err := ReadIndex(root, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != len(layoutFiles) {
		t.Errorf("no HEAD: changed %v", changed)
	}
	run("commit", "-q", "-m", "seed")
	os.WriteFile(filepath.Join(root, "rules/2026-01-15-a.md"), []byte("staged"), 0o644)
	os.WriteFile(filepath.Join(root, "rules/2026-01-15-b.md"), []byte("new"), 0o644)
	os.Remove(filepath.Join(root, "notes.md"))
	run("add", "-A")
	os.WriteFile(filepath.Join(root, "rules/2026-01-15-a.md"), []byte("unstaged"), 0o644)
	idx, changed, err = ReadIndex(root, g)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{"rules/2026-01-15-a.md", "rules/2026-01-15-b.md"}) {
		t.Errorf("changed %v", changed)
	}
	if b, _ := idx.ReadFile("rules/2026-01-15-a.md"); string(b) != "staged" {
		t.Errorf("index content %q", b)
	}
	if idx.Locate("files/2026-01-15-a") != Inside || idx.Locate("files/2026-01-15-a/x.png") != Inside ||
		idx.Locate("notes.md") != Missing || idx.Locate("files/../../x") != Outside {
		t.Error("Locate")
	}
	l, err := Enumerate(idx)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(layoutWant, "notes.md E-NOT-ENTRY\n", "", 1)
	want = strings.Replace(want, "entry rules/2026-01-15-a.md rule 2026-01-15-a\n",
		"entry rules/2026-01-15-a.md rule 2026-01-15-a\nentry rules/2026-01-15-b.md rule 2026-01-15-b\n", 1)
	if got := layoutString(l); got != want {
		t.Errorf("Enumerate(index):\n%s\nwant\n%s", got, want)
	}
}

// setGitEnv gives git a private home and an identity for the duration of a test.
func setGitEnv(t *testing.T, home string) {
	for k, v := range map[string]string{"HOME": home, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.invalid",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.invalid", "GIT_INDEX_FILE": ""} {
		t.Setenv(k, v)
	}
	os.Unsetenv("GIT_INDEX_FILE")
}
