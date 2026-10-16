package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func envOf(m map[string]string) Env { return func(k string) string { return m[k] } }

func TestPath(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"MEMEX_CONFIG": "/c.toml", "XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/c.toml"},
		{map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/x/memex/config.toml"},
		{map[string]string{"HOME": "/h"}, "/h/.config/memex/config.toml"},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := Path(envOf(c.env)); got != c.want {
			t.Errorf("Path(%v) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) Env {
		p := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return envOf(map[string]string{"MEMEX_CONFIG": p})
	}
	if c, err := Load(envOf(nil)); err != nil || c.Path != "" || c.SearchLimit != 12 {
		t.Fatalf("no HOME: %+v, %v", c, err)
	}
	c, err := Load(envOf(map[string]string{"MEMEX_CONFIG": filepath.Join(dir, "absent.toml")}))
	if err != nil || !c.SyncOnPreflight || c.SearchLimit != 12 || c.ListLimit != 50 || c.NetworkTimeout != 20 {
		t.Fatalf("missing file: %+v, %v", c, err)
	}
	c, err = Load(write("store = \"/s\"\nworkspace = \"/w\"\nsync_on_preflight = false\n" +
		"search_limit = 3\nlist_limit = 4\nnetwork_timeout = 5\n"))
	if err != nil || c.Store != "/s" || c.Workspace != "/w" || c.SyncOnPreflight ||
		c.SearchLimit != 3 || c.ListLimit != 4 || c.NetworkTimeout != 5 {
		t.Fatalf("full file: %+v, %v", c, err)
	}
	for text, want := range map[string]string{
		"stor = \"/s\"\n":      `unknown key "stor" (did you mean "store"?)`,
		"store = 1\n":          "store must be a string, not an integer",
		"list_limit = 0\n":     "list_limit must be at least 1, not 0",
		"search_limit = \"3\"": "search_limit must be an integer, not a string",
		"store = \n":           "config file",
	} {
		_, err := Load(write(text))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v, want it to contain %q", text, err, want)
		}
	}
}

func TestExpand(t *testing.T) {
	for in, want := range map[string]string{
		"~/s":         "/h/s",
		"$HOME/s":     "/h/s",
		"${HOME}/s":   "/h/s",
		"$HOME":       "/h",
		"/a/~/b":      "/a/~/b",
		"~user/s":     "~user/s",
		"$USER/s":     "$USER/s",
		"/x/$HOME/y":  "/x//h/y",
		"$HOMEDIR/x":  "$HOMEDIR/x",
		"/a${HOME}b":  "/a/hb",
		"relative/$X": "relative/$X",
		"$HOME/$HOME": "/h//h",
	} {
		if got, err := Expand(in, "/h"); err != nil || got != want {
			t.Errorf("Expand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for in, want := range map[string]string{
		"~/s":       "cannot expand ~: HOME is not set",
		"$HOME/s":   "cannot expand $HOME: HOME is not set",
		"${HOME}/s": "cannot expand ${HOME}: HOME is not set",
	} {
		if _, err := Expand(in, ""); err == nil || err.Error() != want {
			t.Errorf("Expand(%q) without HOME: %v, want %q", in, err, want)
		}
	}
	if got, err := Expand("/abs/$HOMEDIR", ""); err != nil || got != "/abs/$HOMEDIR" {
		t.Errorf("no expansion needed without HOME: %q, %v", got, err)
	}
}

func TestResolveStore(t *testing.T) {
	store := t.TempDir()
	os.WriteFile(filepath.Join(store, "memex.toml"), []byte("format = 1\n"), 0o644)
	empty := t.TempDir()
	home := filepath.Dir(store)
	rel := "~/" + filepath.Base(store)
	cfg := &Config{Path: "/cfg.toml", Store: store}
	env := envOf(map[string]string{"HOME": home})
	envStore := envOf(map[string]string{"HOME": home, "MEMEX_STORE": empty})
	check := func(name string, got string, err error, want, wantErr string) {
		t.Helper()
		if wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Errorf("%s: error %v, want %q", name, err, wantErr)
			}
			return
		}
		if err != nil || got != want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, want)
		}
	}
	got, err := ResolveStore(rel, true, envStore, cfg)
	check("flag wins", got, err, store, "")
	got, err = ResolveStore("", false, envStore, cfg)
	check("env before config", got, err, "", "has no memex.toml")
	got, err = ResolveStore("", false, env, cfg)
	check("config", got, err, store, "")
	got, err = ResolveStore("", false, env, &Config{Path: "/cfg.toml"})
	check("nothing", got, err, "", `no store configured; use --store, MEMEX_STORE, or store = "..." in /cfg.toml`)
	got, err = ResolveStore("s", true, env, cfg)
	check("relative", got, err, "", "relative")
	got, err = ResolveStore(filepath.Join(store, "nope"), true, env, cfg)
	check("missing", got, err, "", "no such file")
}

func TestToday(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.Local)
	got, err := Today(envOf(nil), now)
	if err != nil || got.Format(time.DateOnly) != "2026-03-04" {
		t.Errorf("Today() = %v, %v", got, err)
	}
	got, err = Today(envOf(map[string]string{"MEMEX_TODAY": "2026-01-15"}), now)
	if err != nil || got.Format(time.DateOnly) != "2026-01-15" {
		t.Errorf("MEMEX_TODAY: %v, %v", got, err)
	}
	for _, bad := range []string{"2026-1-15", "2026-02-30", "yesterday", "2026-01-15T00:00:00Z"} {
		if _, err := Today(envOf(map[string]string{"MEMEX_TODAY": bad}), now); err == nil {
			t.Errorf("MEMEX_TODAY=%q accepted", bad)
		}
	}
}

func TestResolveWorkspace(t *testing.T) {
	root := t.TempDir()
	env := func(k string) string {
		if k == "HOME" {
			return root
		}
		return ""
	}
	ws := filepath.Join(root, "ws")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveWorkspace(env, &Config{Workspace: "~/ws/"}, ""); err != nil || got != ws {
		t.Errorf("trailing slash: %q %v", got, err)
	}
	if err := os.Chmod(ws, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(ws, 0o755)
	if os.Geteuid() != 0 { // root reads any directory
		_, err := ResolveWorkspace(env, &Config{Workspace: ws}, "")
		if err == nil || err.Error() != "workspace "+ws+": permission denied" {
			t.Errorf("unreadable workspace: %v", err)
		}
	}
	os.Chmod(ws, 0o755)
	store := filepath.Join(root, "store")
	if err := os.MkdirAll(filepath.Join(store, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{store, filepath.Join(store, "rules")} {
		_, err := ResolveWorkspace(env, &Config{Workspace: w}, store)
		if err == nil || err.Error() != "workspace "+w+" is inside the store "+store {
			t.Errorf("inside the store: %v", err)
		}
	}
	if _, err := ResolveWorkspace(env, &Config{Workspace: ws}, store); err != nil {
		t.Errorf("beside the store: %v", err)
	}
}

func TestPathAlias(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"/h/a/b.md":         "~/a/b.md",
		"~/a/b.md":          "~/a/b.md",
		"$HOME/a/b.md":      "~/a/b.md",
		"${HOME}/a/./b.md":  "~/a/b.md",
		"/h/a/../a//b.md":   "~/a/b.md",
		"/hx/b.md":          "/hx/b.md",
		"/elsewhere/./b.md": "/elsewhere/b.md",
		"rel/b.md":          filepath.Join(wd, "rel/b.md"),
	} {
		if got := PathAlias(in, "/h/"); got != want {
			t.Errorf("PathAlias(%q) = %q, want %q", in, got, want)
		}
	}
	if got := PathAlias("~/b.md", ""); got != "~/b.md" {
		t.Errorf("PathAlias without HOME = %q, want it as given", got)
	}
	for _, c := range []struct {
		alias, path string
		want        bool
	}{
		{"~/a/b.md", "/h/a/b.md", true},
		{"/h/a/b.md", "~/a/b.md", true},
		{"/h/a/b.md", "$HOME/a/b.md", true},
		{"docs/b.md", "docs/b.md", true},
		{"~/a/c.md", "/h/a/b.md", false},
		{"a/b.md", "/h/a/b.md", false}, // a store name, not a file path
	} {
		if got := AliasNamesPath(c.alias, c.path, "/h"); got != c.want {
			t.Errorf("AliasNamesPath(%q, %q) = %v, want %v", c.alias, c.path, got, c.want)
		}
	}
}
