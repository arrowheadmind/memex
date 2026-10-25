package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/neutral"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// Doctor results (SPEC §11.13).
const (
	pass = "PASS"
	warn = "WARN"
	fail = "FAIL"
)

// fetchAgeLimit is the age of FETCH_HEAD past which fetch-age warns (SPEC §11.13).
const fetchAgeLimit = 8 * time.Hour

// minGit is the oldest git doctor accepts (SPEC §11.13).
var minGit = [2]int{2, 30}

type doctorCheck struct {
	id, result, message string
	details             []string
}

// Doctor runs `memex doctor` (SPEC §11.13). It is read-only and needs no store.
func Doctor(e *Env, asJSON bool) int {
	var checks []doctorCheck
	add := func(id, result, format string, args ...any) *doctorCheck {
		checks = append(checks, doctorCheck{id: id, result: result, message: fmt.Sprintf(format, args...)})
		return &checks[len(checks)-1]
	}
	home := e.Getenv("HOME")

	// git
	gitOK := false
	if out, err := gitx.New("/", 10*time.Second, noLocks).Run("version"); err != nil {
		add("git", fail, "git is not runnable: %s", firstLine(err.Error()))
	} else if v, ok := gitVersion(out); !ok {
		add("git", fail, "cannot read the git version from %q", strings.TrimSpace(out))
	} else if v[0] < minGit[0] || (v[0] == minGit[0] && v[1] < minGit[1]) {
		add("git", fail, "git %d.%d is older than %d.%d", v[0], v[1], minGit[0], minGit[1])
	} else {
		gitOK = true
		add("git", pass, "version %d.%d or later", minGit[0], minGit[1])
	}

	// config
	cfg, cfgErr := config.Load(e.Getenv)
	today, todayErr := config.Today(e.Getenv, e.Now())
	switch {
	case cfgErr != nil:
		add("config", fail, "%s", cfgErr.Error())
		cfg = &config.Config{Path: config.Path(e.Getenv)} // resolve the store without the file
	case todayErr != nil:
		add("config", fail, "%s", todayErr.Error())
	default:
		if _, err := os.Stat(cfg.Path); cfg.Path == "" || err != nil {
			add("config", pass, "no config file")
		} else {
			add("config", pass, "config file parses")
		}
	}
	if todayErr != nil {
		today, _ = config.Today(func(string) string { return "" }, e.Now())
	}

	// store, then the checks that need it
	root, err := config.ResolveStore(e.Store, e.StoreSet, e.Getenv, cfg)
	if err != nil {
		add("store", fail, "%s", err.Error())
		root = ""
	} else {
		m, findings, ferr := store.ReadManifest(store.WorkTree{Root: root})
		var fe *store.FormatError
		switch {
		case errors.As(ferr, &fe):
			add("store", pass, "%s", storeLine(root, m))
			add("format", fail, "%s", fe.Msg)
		case ferr != nil:
			add("store", fail, "store %s: %v", root, unwrap(ferr))
			root = ""
		case len(findings) > 0:
			add("store", fail, "%s: %s %s", findings[0].Path, findings[0].Code, findings[0].Message)
			root = ""
		default:
			add("store", pass, "%s", storeLine(root, m))
			add("format", pass, "format %d", m.Format)
		}
	}
	storeOK := root != "" && checks[len(checks)-1].result == pass
	if storeOK && gitOK {
		e.doctorGit(root, add)
	}
	if storeOK {
		hist, err := store.ReadHistory(gitx.New(root, 10*time.Second, noLocks), check.CurrencyDirs())
		var findings []check.Finding
		if err == nil {
			findings, err = check.RunHistory(store.WorkTree{Root: root}, today, nil, hist)
		}
		if err != nil {
			add("check", fail, "%v", err)
		} else {
			errs, warns := 0, 0
			for _, f := range findings {
				if check.IsError(f) {
					errs++
				} else {
					warns++
				}
			}
			if errs > 0 {
				add("check", fail, "%d errors, %d warnings; run memex check", errs, warns)
			} else {
				add("check", pass, "%d errors, %d warnings", errs, warns)
			}
		}
	}

	// path
	e.doctorPath(add)

	// leaks
	scan := neutral.Scan{Home: home, Store: root}
	wsNote := ""
	if cfgErr == nil && cfg.Workspace != "" {
		ws, err := config.ResolveWorkspace(e.Getenv, cfg, root)
		if err != nil {
			wsNote = "; the workspace was not scanned: " + err.Error()
		} else {
			scan.Workspace = ws
		}
	}
	leaks := scan.Leaks()
	if len(leaks) == 0 && wsNote == "" {
		add("leaks", pass, "no files in coding tools' own memory")
	} else if len(leaks) == 0 {
		add("leaks", warn, "no files in coding tools' own memory%s", wsNote)
	} else {
		c := add("leaks", warn, "%d %s in coding tools' own memory; move each into the store with memex capture --from PATH, then delete it%s",
			len(leaks), plural(len(leaks), "file", "files"), wsNote)
		for _, p := range leaks {
			c.details = append(c.details, "LEAK "+p)
		}
	}

	// tool-memory
	if ok, msg := neutral.ToolMemory(e.Getenv, home); ok {
		add("tool-memory", pass, "%s", msg)
	} else {
		add("tool-memory", warn, "%s", msg)
	}

	failed := 0
	for _, c := range checks {
		if c.result == fail {
			failed++
		}
	}
	if asJSON {
		items := jsonx.Array{}
		for _, c := range checks {
			items = append(items, jsonx.Object{
				{Key: "id", Value: jsonx.String(c.id)},
				{Key: "result", Value: jsonx.String(c.result)},
				{Key: "message", Value: jsonx.String(c.message)},
				{Key: "details", Value: jsonx.Strings(c.details)},
			})
		}
		e.Stdout.Write(jsonx.Encode(jsonx.Object{{Key: "checks", Value: items}, {Key: "failed", Value: jsonx.Int(failed)}}))
	} else {
		var b strings.Builder
		for _, c := range checks {
			fmt.Fprintf(&b, "%s %s %s\n", c.result, c.id, c.message)
			for _, d := range c.details {
				b.WriteString(d + "\n")
			}
		}
		e.Stdout.Write([]byte(b.String()))
	}
	if failed > 0 {
		return spec.ExitFailure
	}
	return spec.ExitOK
}

func storeLine(root string, m *store.Manifest) string {
	if m != nil && m.Name != "" {
		return fmt.Sprintf("%s, name %s", root, entry.Quote(m.Name))
	}
	return root
}

var gitVersionRe = regexp.MustCompile(`git version (\d+)\.(\d+)`)

func gitVersion(out string) ([2]int, bool) {
	m := gitVersionRe.FindStringSubmatch(out)
	if m == nil {
		return [2]int{}, false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return [2]int{a, b}, true
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// noLocks keeps every git command of doctor from taking optional locks, such as the index
// refresh of git status, so that doctor writes nothing (SPEC §11.13).
var noLocks = gitx.Extra("GIT_OPTIONAL_LOCKS=0")

// doctorGit runs the upstream, fetch-age, clean and hooks checks (SPEC §11.13).
func (e *Env) doctorGit(root string, add func(id, result, format string, args ...any) *doctorCheck) {
	g := gitx.New(root, 10*time.Second, noLocks)
	run := func(args ...string) (string, bool) {
		out, err := g.Run(args...)
		return strings.TrimSpace(out), err == nil
	}
	// A store that is not the top of its own repository has no upstream, fetch or hooks of
	// its own: one WARN, and the other git checks are left out.
	top, ok := run("rev-parse", "--show-toplevel")
	real, err := filepath.EvalSymlinks(root)
	if !ok || err != nil || filepath.Clean(top) != filepath.Clean(real) {
		add("upstream", warn, "%s is not a git repository; run memex init %s", root, root)
		return
	}
	gitPath := func(name string) string {
		p, ok := run("rev-parse", "--git-path", name)
		if !ok {
			return ""
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return p
	}

	// upstream
	hasUpstream := false
	branch, onBranch := run("symbolic-ref", "-q", "--short", "HEAD")
	switch {
	case !onBranch:
		add("upstream", warn, "HEAD is not on a branch; run git -C %s switch main", root)
	default:
		if up, ok := run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); ok && up != "" {
			hasUpstream = true
			add("upstream", pass, "%s tracks %s", branch, up)
		} else {
			add("upstream", warn, "branch %s has no upstream, so nothing is pushed; run git -C %s push -u origin %s", branch, root, branch)
		}
	}

	// fetch-age
	if p := gitPath("FETCH_HEAD"); p == "" {
		add("fetch-age", warn, "never fetched; run memex sync")
	} else if st, err := os.Stat(p); err != nil {
		add("fetch-age", warn, "never fetched; run memex sync")
	} else {
		at := st.ModTime().UTC().Truncate(time.Second)
		if age := e.Now().Sub(at); age > fetchAgeLimit {
			add("fetch-age", warn, "last fetch %s, more than %d hours ago; run memex sync", at.Format(time.RFC3339), int(fetchAgeLimit/time.Hour))
		} else {
			add("fetch-age", pass, "last fetch %s", at.Format(time.RFC3339))
		}
	}

	// clean
	var problems []string
	if out, ok := run("status", "--porcelain", "--untracked-files=all"); !ok {
		problems = append(problems, "git status failed")
	} else if out != "" {
		n := len(strings.Split(out, "\n"))
		problems = append(problems, fmt.Sprintf("%d uncommitted %s; run memex save", n, plural(n, "change", "changes")))
	}
	if hasUpstream {
		if out, ok := run("rev-list", "--count", "@{u}..HEAD"); ok && out != "0" {
			n, _ := strconv.Atoi(out)
			problems = append(problems, fmt.Sprintf("%d unpushed %s; run memex sync", n, plural(n, "commit", "commits")))
		}
	}
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		if p := gitPath(name); p != "" {
			if _, err := os.Stat(p); err == nil {
				problems = append(problems, "a rebase is in progress; finish it with git -C "+root+" rebase --continue, or --abort")
				break
			}
		}
	}
	if len(problems) > 0 {
		add("clean", warn, "%s", strings.Join(problems, "; "))
	} else {
		add("clean", pass, "nothing uncommitted or unpushed")
	}

	// hooks
	hooksPath, _ := run("config", "core.hooksPath")
	hook := filepath.Join(root, ".githooks", "pre-commit")
	st, err := os.Stat(hook)
	switch {
	case hooksPath != ".githooks" && hooksPath == "":
		add("hooks", warn, "core.hooksPath is not set, so the pre-commit check does not run; run memex init %s", root)
	case hooksPath != ".githooks":
		add("hooks", warn, "core.hooksPath is %s, not .githooks; run git -C %s config core.hooksPath .githooks", entry.Quote(hooksPath), root)
	case err != nil || !st.Mode().IsRegular() || st.Mode()&0o111 == 0:
		add("hooks", warn, ".githooks/pre-commit is missing or not executable; run memex init %s", root)
	default:
		add("hooks", pass, "core.hooksPath is .githooks and the pre-commit hook exists")
	}
}

// doctorPath is the path check (SPEC §11.13): every memex executable on PATH resolves to
// the same file.
func (e *Env) doctorPath(add func(id, result, format string, args ...any) *doctorCheck) {
	var found, resolved []string
	for _, dir := range filepath.SplitList(e.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		p := filepath.Join(dir, "memex")
		st, err := os.Stat(p)
		if err != nil || !st.Mode().IsRegular() || st.Mode()&0o111 == 0 {
			continue
		}
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			r = p
		}
		found = append(found, p)
		resolved = append(resolved, r)
	}
	switch {
	case len(found) == 0:
		add("path", warn, "no memex on PATH; hooks run memex by name")
	default:
		same := true
		for _, r := range resolved[1:] {
			if r != resolved[0] {
				same = false
			}
		}
		if same {
			if len(found) == 1 {
				add("path", pass, "1 memex on PATH")
			} else {
				add("path", pass, "%d memex on PATH, all the same file", len(found))
			}
		} else {
			add("path", warn, "memex on PATH resolves to different files: %s; the first one runs", strings.Join(found, ", "))
		}
	}
}
