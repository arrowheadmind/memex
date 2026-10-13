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
	c, err := Load(envOf(map[string]string{"MEMEX_CONFIG": filepath.Join(dir, "absent.toml")}))
	if err != nil || !c.SyncOnPreflight || c.SearchLimit != 12 || c.ListLimit != 50 || c.NetworkTimeout != 20 {
		t.Fatalf("missing file: %+v, %v", c, err)
	}
	c, err = Load(write("store = \"/s\"\nhost = \"h\"\nworkspace = \"/w\"\nsync_on_preflight = false\n" +
		"search_limit = 3\nlist_limit = 4\nnetwork_timeout = 5\n"))
	if err != nil || c.Store != "/s" || c.Host != "h" || c.Workspace != "/w" || c.SyncOnPreflight ||
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
		"/a/~/b":      "/a/~/b",
		"~user/s":     "~user/s",
		"$USER/s":     "$USER/s",
		"/x/$HOME/y":  "/x//h/y",
		"relative/$X": "relative/$X",
	} {
		if got := Expand(in, "/h"); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
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

func TestHost(t *testing.T) {
	hn := func() (string, error) { return "Box.example.org", nil }
	if h := Host(envOf(map[string]string{"MEMEX_HOST": "m"}), &Config{Host: "c"}, hn); h != "m" {
		t.Errorf("MEMEX_HOST: %q", h)
	}
	if h := Host(envOf(nil), &Config{Host: "c"}, hn); h != "c" {
		t.Errorf("config: %q", h)
	}
	if h := Host(envOf(nil), &Config{}, hn); h != "box" {
		t.Errorf("hostname: %q", h)
	}
}
