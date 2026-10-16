// Package config implements SPEC §3: the config file, the environment variables and store
// resolution.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/arrowheadmind/memex/internal/spec"
)

// Error is a configuration error (SPEC §10 exit 2). Its message is printed after "memex: ".
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func errorf(format string, args ...any) *Error { return &Error{fmt.Sprintf(format, args...)} }

// Env reads one environment variable; "" means unset or empty.
type Env func(string) string

// Config holds the settings of SPEC §3.2, with defaults applied.
type Config struct {
	Path            string // the config file path, whether or not it exists
	Store           string // "" when not set
	Workspace       string // "" when not set
	SyncOnPreflight bool
	SearchLimit     int
	ListLimit       int
	NetworkTimeout  int // seconds
}

// keys is the SPEC §3.2 table: key and TOML kind.
var keys = []struct{ name, kind string }{
	{"store", "String"},
	{"workspace", "String"},
	{"sync_on_preflight", "Bool"},
	{"search_limit", "Integer"},
	{"list_limit", "Integer"},
	{"network_timeout", "Integer"},
}

// Path returns the config file path (SPEC §3.2), or "" when neither XDG_CONFIG_HOME nor HOME
// is set, in which case there is no config file.
func Path(env Env) string {
	if p := env("MEMEX_CONFIG"); p != "" {
		return p
	}
	if x := env("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "memex", "config.toml")
	}
	if h := env("HOME"); h != "" {
		return filepath.Join(h, ".config", "memex", "config.toml")
	}
	return ""
}

// displayPath names the config file in messages.
func (c *Config) displayPath() string {
	if c.Path == "" {
		return "~/.config/memex/config.toml"
	}
	return c.Path
}

// Load reads the config file. A missing file gives the defaults; a file that won't parse,
// has an unknown key or a value of the wrong kind is an *Error.
func Load(env Env) (*Config, error) {
	c := &Config{Path: Path(env), SyncOnPreflight: true, SearchLimit: 12, ListLimit: 50, NetworkTimeout: 20}
	if c.Path == "" {
		return c, nil
	}
	data, err := os.ReadFile(c.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, errorf("config file %s: %v", c.Path, unwrapPath(err))
	}
	var raw map[string]any
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, errorf("config file %s: %v", c.Path, err)
	}
	var names []string
	for _, k := range keys {
		names = append(names, k.name)
	}
	present := make([]string, 0, len(raw))
	for k := range raw {
		present = append(present, k)
	}
	slices.Sort(present)
	for _, k := range present {
		i := slices.Index(names, k)
		if i < 0 {
			return nil, errorf("config file %s: unknown key %q%s", c.Path, k, suggest(k, names))
		}
		if got := md.Type(k); got != keys[i].kind {
			return nil, errorf("config file %s: %s must be %s, not %s", c.Path, k, kindName(keys[i].kind), kindName(got))
		}
		switch v := raw[k].(type) {
		case string:
			switch k {
			case "store":
				c.Store = v
			case "workspace":
				c.Workspace = v
			}
		case bool:
			c.SyncOnPreflight = v
		case int64:
			if v < 1 {
				return nil, errorf("config file %s: %s must be at least 1, not %d", c.Path, k, v)
			}
			n := int(min(v, 1<<31-1))
			switch k {
			case "search_limit":
				c.SearchLimit = n
			case "list_limit":
				c.ListLimit = n
			case "network_timeout":
				c.NetworkTimeout = n
			}
		}
	}
	return c, nil
}

func kindName(tomlType string) string {
	switch tomlType {
	case "String":
		return "a string"
	case "Bool":
		return "a boolean"
	case "Integer":
		return "an integer"
	case "Float":
		return "a float"
	case "Array":
		return "an array"
	case "Hash", "Table":
		return "a table"
	}
	return "a " + strings.ToLower(tomlType)
}

func suggest(name string, candidates []string) string {
	if near, ok := spec.Nearest(name, candidates); ok {
		return fmt.Sprintf(" (did you mean %q?)", near)
	}
	return ""
}

func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

var dollarHome = regexp.MustCompile(`\$HOME(/|$)`)

// Expand applies the SPEC §3.1 expansion: a leading "~/", "${HOME}" anywhere, and "$HOME"
// where it is followed by "/" or ends the path. Nothing else is expanded and symlinks are
// not resolved. Expanding with HOME unset is an *Error.
func Expand(path, home string) (string, error) {
	unset := func(what string) error { return errorf("cannot expand %s: HOME is not set", what) }
	if strings.HasPrefix(path, "~/") {
		if home == "" {
			return "", unset("~")
		}
		path = home + path[1:]
	}
	if strings.Contains(path, "${HOME}") {
		if home == "" {
			return "", unset("${HOME}")
		}
		path = strings.ReplaceAll(path, "${HOME}", home)
	}
	if dollarHome.MatchString(path) {
		if home == "" {
			return "", unset("$HOME")
		}
		path = dollarHome.ReplaceAllStringFunc(path, func(m string) string { return home + m[len("$HOME"):] })
	}
	return path, nil
}

// PathAlias returns the alias form of a file path (SPEC §11.2): expanded as in §3.1, made
// absolute against the working directory and cleaned lexically, with the home directory
// written "~/" when the file is under it. Symbolic links are not resolved. A path that cannot
// be expanded or made absolute is returned as given.
func PathAlias(path, home string) string {
	p, err := Expand(path, home)
	if err != nil {
		return path
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return path
	}
	if home != "" {
		if h := filepath.Clean(home); h != "/" && strings.HasPrefix(abs, h+"/") {
			return "~/" + abs[len(h)+1:]
		}
	}
	return abs
}

// AliasNamesPath reports whether alias names the file at path (SPEC §11.2): alias is path as
// given, or alias is itself a file path (absolute, or beginning with "~/", "$HOME" or
// "${HOME}") with the same alias form. Other aliases are store names, not file paths, and are
// compared as given only.
func AliasNamesPath(alias, path, home string) bool {
	if alias == path {
		return true
	}
	if !strings.HasPrefix(alias, "/") && !strings.HasPrefix(alias, "~/") &&
		!strings.HasPrefix(alias, "$HOME") && !strings.HasPrefix(alias, "${HOME}") {
		return false
	}
	return PathAlias(alias, home) == PathAlias(path, home)
}

// ResolveStore returns the store root (SPEC §3.1). flag is the --store value and flagSet
// whether it was given.
func ResolveStore(flag string, flagSet bool, env Env, c *Config) (string, error) {
	var path string
	switch {
	case flagSet:
		path = flag
	case env("MEMEX_STORE") != "":
		path = env("MEMEX_STORE")
	case c.Store != "":
		path = c.Store
	default:
		return "", errorf("no store configured; use --store, MEMEX_STORE, or store = \"...\" in %s", c.displayPath())
	}
	path, err := Expand(path, env("HOME"))
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errorf("store path %q is relative; give an absolute path", path)
	}
	path = filepath.Clean(path) // a trailing / or a /./ names the same store
	info, err := os.Stat(path)
	if err != nil {
		return "", errorf("store %s: %v", path, unwrapPath(err))
	}
	if !info.IsDir() {
		return "", errorf("store %s is not a directory", path)
	}
	if info, err := os.Stat(filepath.Join(path, "memex.toml")); err != nil || !info.Mode().IsRegular() {
		return "", errorf("store %s has no memex.toml; it is not a memex store (create one with memex init)", path)
	}
	return path, nil
}

var datePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// ParseDate parses a YYYY-MM-DD calendar date.
func ParseDate(s string) (time.Time, bool) {
	if !datePattern.MatchString(s) {
		return time.Time{}, false
	}
	t, err := time.Parse(time.DateOnly, s)
	return t, err == nil
}

// Today returns today's date (SPEC §2): MEMEX_TODAY if set, else the local date. A malformed
// MEMEX_TODAY is an *Error.
func Today(env Env, now time.Time) (time.Time, error) {
	if s := env("MEMEX_TODAY"); s != "" {
		t, ok := ParseDate(s)
		if !ok {
			return time.Time{}, errorf("MEMEX_TODAY=%q is not a date; use YYYY-MM-DD", s)
		}
		return t, nil
	}
	y, m, d := now.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
}

// ResolveWorkspace returns the configured workspace (SPEC §3.2, §12), expanded as §3.1
// expands the store path. No `workspace` key is an *Error naming the config file, and so is
// a relative path, one that is not a readable directory, and one inside the store at root
// (when root is not "").
func ResolveWorkspace(env Env, c *Config, root string) (string, error) {
	if c.Workspace == "" {
		return "", errorf("no workspace configured; set workspace = \"...\" in %s", c.displayPath())
	}
	path, err := Expand(c.Workspace, env("HOME"))
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errorf("workspace path %q is relative; give an absolute path", path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", errorf("workspace %s: %v", path, unwrapPath(err))
	}
	if !info.IsDir() {
		return "", errorf("workspace %s is not a directory", path)
	}
	if _, err := os.ReadDir(path); err != nil {
		return "", errorf("workspace %s: %v", path, unwrapPath(err))
	}
	if root != "" && inside(path, root) {
		return "", errorf("workspace %s is inside the store %s", path, root)
	}
	return path, nil
}

// inside reports whether path is dir or below it, comparing the cleaned paths and, when
// they resolve, the paths with symbolic links resolved.
func inside(path, dir string) bool {
	below := func(p, d string) bool {
		return p == d || strings.HasPrefix(p, strings.TrimSuffix(d, "/")+"/")
	}
	if below(filepath.Clean(path), filepath.Clean(dir)) {
		return true
	}
	rp, err1 := filepath.EvalSymlinks(path)
	rd, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && below(rp, rd)
}
