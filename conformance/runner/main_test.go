package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPattern(t *testing.T) {
	subst := map[string]string{"STORE": "/tmp/s.x", "HOME": "/h", "VERSION": "0.0.0-dev"}
	cases := []struct {
		want, got string
		ok        bool
	}{
		{"pushed <SHA7>\n", "pushed 0a1b2c3\n", true},
		{"pushed <SHA7>\n", "pushed 0A1B2C3\n", false},
		{"pushed <SHA7>\n", "pushed 0a1b2c34\n", false},
		{"fetched <TIME>\n", "fetched 2026-01-15T10:00:00Z\n", true},
		{"fetched <TIME>\n", "fetched 2026-01-15T10:00:00.5+02:00\n", true},
		{"fetched <TIME>\n", "fetched yesterday\n", false},
		{"Store root: <STORE>\n", "Store root: /tmp/s.x\n", true},
		{"Store root: <STORE>\n", "Store root: /tmp/sXx\n", false},
		{"<HOME>/a <VERSION>", "/h/a 0.0.0-dev", true},
		{"a.b\n", "a.b\n", true},
		{"a.b\n", "axb\n", false},
		{"a\n", "a\n\n", false},
		{"<OTHER>", "<OTHER>", true},
	}
	for _, c := range cases {
		if got := pattern(c.want, subst, true).MatchString(c.got); got != c.ok {
			t.Errorf("pattern(%q) on %q = %v, want %v", c.want, c.got, got, c.ok)
		}
	}
}

// fakeMemex prints the version, or commits a file to the store and prints the outcome line.
const fakeMemex = `#!/bin/sh
case "$1" in
version) echo "memex 9.9.9 (store format 1)" ;;
write)
  cd "$MEMEX_STORE" || exit 1
  printf 'x\n' > rules/new.md
  git add rules/new.md && git commit -q -m "memex: new rule new" || exit 1
  echo "committed $(git rev-parse --short=7 HEAD), no upstream"
  echo "root $MEMEX_STORE home $HOME today $MEMEX_TODAY" >&2
  ;;
*) echo "bad" >&2; exit 2 ;;
esac
`

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestRunAllAgainstFake(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	memex := filepath.Join(dir, "memex")
	writeFile(t, memex, fakeMemex, 0o755)
	cases := filepath.Join(dir, "cases")

	writeFile(t, filepath.Join(cases, "001-version/cmd"), "version\n", 0o644)
	writeFile(t, filepath.Join(cases, "001-version/expect/stdout"), "memex <VERSION> (store format 1)\n", 0o644)
	writeFile(t, filepath.Join(cases, "001-version/expect/exit"), "0\n", 0o644)

	c := filepath.Join(cases, "002-write")
	writeFile(t, filepath.Join(c, "store/memex.toml"), "format = 1\n", 0o644)
	writeFile(t, filepath.Join(c, "store/rules/old.md"), "old\n", 0o644)
	writeFile(t, filepath.Join(c, "store/.githooks/pre-commit"), "#!/bin/sh\nexit 0\n", 0o755)
	writeFile(t, filepath.Join(c, "steps/01/cmd"), "write\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/stdout"), "committed <SHA7>, no upstream\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/exit"), "0\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/stderr-contains"), "root <STORE> home <HOME>\ntoday 2026-01-15\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/commits"), "memex: new rule new\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/store/memex.toml"), "format = 1\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/store/rules/old.md"), "old\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/store/rules/new.md"), "x\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/01/expect/store/.githooks/pre-commit"), "#!/bin/sh\nexit 0\n", 0o755)
	writeFile(t, filepath.Join(c, "steps/02/cmd"), "nope\n", 0o644)
	writeFile(t, filepath.Join(c, "steps/02/expect/stdout"), "", 0o644)
	writeFile(t, filepath.Join(c, "steps/02/expect/exit"), "2\n", 0o644)

	// A failing case: wrong stdout, wrong exit, wrong executable bit.
	f := filepath.Join(cases, "003-fails")
	writeFile(t, filepath.Join(f, "store/a"), "a\n", 0o644)
	writeFile(t, filepath.Join(f, "cmd"), "version\n", 0o644)
	writeFile(t, filepath.Join(f, "expect/stdout"), "memex 1.0.0 (store format 1)\n", 0o644)
	writeFile(t, filepath.Join(f, "expect/exit"), "1\n", 0o644)
	writeFile(t, filepath.Join(f, "expect/store/a"), "a\n", 0o755)

	out := captureStdout(t, func() {
		code, err := runAll(memex, cases, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		if code != 1 {
			t.Errorf("exit %d, want 1", code)
		}
	})
	for _, want := range []string{
		"PASS 001-version\n",
		"PASS 002-write\n",
		"FAIL 003-fails\n",
		"exit 0, want 1",
		"stdout differs",
		"store: a executable bit is false, want true",
		"conformance: 2 passed, 1 failed\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()
	f()
	w.Close()
	os.Stdout = saved
	return <-done
}
