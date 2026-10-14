// Command runner runs the SPEC §15 conformance suite against a memex executable:
//
//	go run ./conformance/runner --memex ./memex [--cases conformance/cases] [--keep] [NAME…]
//
// Each NAME selects the cases whose directory name contains it. The runner only drives the
// executable; it shares no code with the implementation.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	seedMessage    = "conformance: seed"
	seedDate       = "2026-01-10T12:00:00+00:00" // author and committer date of src/ seeds
	seedDate2      = "2026-01-11T12:00:00+00:00" // the same for src2/ second commits
	commandTimeout = 120 * time.Second
)

func main() {
	memex := flag.String("memex", "", "path of the memex executable under test (required)")
	casesDir := flag.String("cases", "conformance/cases", "directory holding the cases")
	keep := flag.Bool("keep", false, "keep each case's temporary directory and print its path")
	flag.Parse()
	if *memex == "" {
		fmt.Fprintln(os.Stderr, "runner: --memex PATH is required")
		os.Exit(2)
	}
	code, err := runAll(*memex, *casesDir, *keep, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "runner: %v\n", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func runAll(memex, casesDir string, keep bool, filters []string) (int, error) {
	memex, err := filepath.Abs(memex)
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(memex); err != nil {
		return 0, err
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return 0, fmt.Errorf("git not found: %w", err)
	}
	if git, err = filepath.Abs(git); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		return 0, err
	}
	var cases []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if len(filters) > 0 && !slices.ContainsFunc(filters, func(f string) bool {
			return strings.Contains(e.Name(), f)
		}) {
			continue
		}
		cases = append(cases, e.Name())
	}
	version, err := memexVersion(memex, git)
	if err != nil {
		return 0, err
	}
	passed, failed := 0, 0
	for _, name := range cases {
		problems, tmp, err := runCase(filepath.Join(casesDir, name), memex, git, version)
		if err != nil {
			problems = append(problems, err.Error())
		}
		if keep {
			fmt.Printf("     %s: kept %s\n", name, tmp)
		} else if tmp != "" {
			os.RemoveAll(tmp)
		}
		if len(problems) == 0 {
			passed++
			fmt.Printf("PASS %s\n", name)
			continue
		}
		failed++
		fmt.Printf("FAIL %s\n", name)
		for _, p := range problems {
			fmt.Printf("     %s\n", strings.ReplaceAll(p, "\n", "\n     "))
		}
	}
	fmt.Printf("conformance: %d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}

// sandbox is one case's temporary world (SPEC §15, "Runner environment").
type sandbox struct {
	root, home, store, bin, config string
	env                            []string
	seed                           string            // hash of the seed commit, "" without store/
	srcs                           map[string]string // src/<name> repositories, by name
	listener                       net.Listener      // the origin-unreachable listener, or nil
	caseDir                        string
}

// listenSilently opens a local TCP listener that accepts every connection and never
// answers, so that a git fetch, pull or push to it hangs until memex's network_timeout
// (SPEC §15, origin-unreachable). The connections stay open until the listener is closed.
func listenSilently() (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	go func() {
		var conns []net.Conn
		defer func() {
			for _, c := range conns {
				c.Close()
			}
		}()
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			conns = append(conns, c)
		}
	}()
	return l, nil
}

func newSandbox(memex, git string, unreachable bool) (*sandbox, error) {
	root, err := os.MkdirTemp("", "memex-conformance-")
	if err != nil {
		return nil, err
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return nil, err
	}
	s := &sandbox{
		root:   root,
		home:   filepath.Join(root, "home"),
		store:  filepath.Join(root, "store"),
		bin:    filepath.Join(root, "bin"),
		config: filepath.Join(root, "config.toml"),
	}
	for _, d := range []string{s.home, s.store, s.bin} {
		if err := os.Mkdir(d, 0o755); err != nil {
			return s, err
		}
	}
	if err := os.Symlink(memex, filepath.Join(s.bin, "memex")); err != nil {
		return s, err
	}
	if err := os.Symlink(git, filepath.Join(s.bin, "git")); err != nil {
		return s, err
	}
	gitconfig := filepath.Join(s.home, ".gitconfig")
	err = os.WriteFile(gitconfig, []byte("[user]\n\tname = test\n\temail = test@example.invalid\n"+
		"[init]\n\tdefaultBranch = main\n"), 0o644)
	if err != nil {
		return s, err
	}
	config := ""
	if unreachable {
		config = "network_timeout = 2\n"
	}
	if err := os.WriteFile(s.config, []byte(config), 0o644); err != nil {
		return s, err
	}
	s.env = []string{
		"PATH=" + s.bin,
		"HOME=" + s.home,
		"GIT_CONFIG_GLOBAL=" + gitconfig,
		"GIT_CONFIG_NOSYSTEM=1",
		"MEMEX_STORE=" + s.store,
		"MEMEX_CONFIG=" + s.config,
		"MEMEX_TODAY=2026-01-15",
		"TZ=UTC",
		"LC_ALL=C",
	}
	return s, nil
}

// git runs git in dir with the sandbox environment, for the runner's own setup.
func (s *sandbox) git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Path = filepath.Join(s.bin, "git")
	cmd.Dir = dir
	cmd.Env = s.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// seedStore copies src into the store, commits it and adds a bare clone as origin with
// upstream tracking (SPEC §15, store/).
func (s *sandbox) seedStore(src string, unreachable bool) error {
	if err := copyTree(src, s.store); err != nil {
		return err
	}
	steps := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"commit", "-q", "--allow-empty", "-m", seedMessage},
	}
	for _, a := range steps {
		if _, err := s.git(s.store, a...); err != nil {
			return err
		}
	}
	origin := filepath.Join(s.root, "origin.git")
	if _, err := s.git(s.root, "clone", "-q", "--bare", s.store, origin); err != nil {
		return err
	}
	for _, a := range [][]string{
		{"remote", "add", "origin", origin},
		{"fetch", "-q", "origin"},
		{"branch", "-q", "--set-upstream-to=origin/main"},
	} {
		if _, err := s.git(s.store, a...); err != nil {
			return err
		}
	}
	if unreachable {
		l, err := listenSilently()
		if err != nil {
			return err
		}
		s.listener = l
		url := "git://" + l.Addr().String() + "/store.git"
		if _, err := s.git(s.store, "remote", "set-url", "origin", url); err != nil {
			return err
		}
	}
	seed, err := s.git(s.store, "rev-parse", "HEAD")
	s.seed = seed
	return err
}

func memexVersion(memex, git string) (string, error) {
	s, err := newSandbox(memex, git, false)
	if s != nil {
		defer os.RemoveAll(s.root)
	}
	if err != nil {
		return "", err
	}
	res := s.run([]string{"version"}, nil)
	m := regexp.MustCompile(`^memex (\S+) \(store format 1\)\n$`).FindStringSubmatch(res.stdout)
	if res.err != nil || res.exit != 0 || m == nil {
		return "", fmt.Errorf("memex version: exit %d, output %q, %v", res.exit, res.stdout, res.err)
	}
	return m[1], nil
}

type result struct {
	stdout, stderr string
	exit           int
	err            error
}

func (s *sandbox) run(args []string, stdin []byte) result {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(s.bin, "memex"), args...)
	cmd.Dir = s.home
	cmd.Env = s.env
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	r := result{stdout: out.String(), stderr: errb.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
		r.exit = exitErr.ExitCode()
	default:
		r.err = err
	}
	return r
}

func runCase(dir, memex, git, version string) (problems []string, tmp string, err error) {
	unreachable := exists(filepath.Join(dir, "origin-unreachable"))
	s, err := newSandbox(memex, git, unreachable)
	if s != nil {
		tmp = s.root
		s.caseDir = dir
		defer func() {
			if s.listener != nil {
				s.listener.Close()
			}
		}()
	}
	if err != nil {
		return nil, tmp, err
	}
	if exists(filepath.Join(dir, "store")) {
		if err := s.seedStore(filepath.Join(dir, "store"), unreachable); err != nil {
			return nil, tmp, err
		}
	}
	if err := s.seedSources(dir); err != nil {
		return nil, tmp, err
	}
	// home/ holds files copied into the temporary HOME (SPEC §15), for commands that read
	// files outside the store, such as import --leak and the leak scan.
	if exists(filepath.Join(dir, "home")) {
		if err := copyTree(filepath.Join(dir, "home"), s.home); err != nil {
			return nil, tmp, err
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "env")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if line == "" {
				continue
			}
			if !strings.Contains(line, "=") {
				return nil, tmp, fmt.Errorf("env: line %q is not KEY=VALUE", line)
			}
			s.env = append(s.env, s.paths(line))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, tmp, err
	}

	steps := []string{dir}
	if exists(filepath.Join(dir, "steps")) {
		entries, err := os.ReadDir(filepath.Join(dir, "steps"))
		if err != nil {
			return nil, tmp, err
		}
		steps = nil
		for _, e := range entries {
			if e.IsDir() {
				steps = append(steps, filepath.Join(dir, "steps", e.Name()))
			}
		}
	}
	subst := map[string]string{"STORE": s.store, "HOME": s.home, "VERSION": version}
	for _, step := range steps {
		label := ""
		if step != dir {
			label = "step " + filepath.Base(step) + ": "
		}
		ps, err := s.runStep(step, subst)
		if err != nil {
			return problems, tmp, fmt.Errorf("%s%v", label, err)
		}
		for _, p := range ps {
			problems = append(problems, label+p)
		}
	}
	return problems, tmp, nil
}

// paths replaces <STORE> and <HOME> in an env value or argument (SPEC §15).
func (s *sandbox) paths(text string) string {
	text = strings.NewReplacer("<STORE>", s.store, "<HOME>", s.home).Replace(text)
	for name, dir := range s.srcs {
		text = strings.ReplaceAll(text, "<SRC:"+name+">", dir)
	}
	return text
}

// seedSources builds one git repository per directory under the case's src/ (SPEC §15):
// the files are committed with a fixed author and date, so that ids, blame dates and commit
// hashes are the same on every run. The empty store directory is removed when the case has
// no store/, so that a command may create it.
func (s *sandbox) seedSources(caseDir string) error {
	entries, err := os.ReadDir(filepath.Join(caseDir, "src"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	s.srcs = map[string]string{}
	env := append(append([]string(nil), s.env...),
		"GIT_AUTHOR_DATE="+seedDate, "GIT_COMMITTER_DATE="+seedDate)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.root, "src", e.Name())
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := copyTree(filepath.Join(caseDir, "src", e.Name()), dir); err != nil {
			return err
		}
		gitIn := func(env []string, a ...string) error {
			cmd := exec.Command("git", a...)
			cmd.Path = filepath.Join(s.bin, "git")
			cmd.Dir = dir
			cmd.Env = env
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("src %s: git %s: %v\n%s", e.Name(), strings.Join(a, " "), err, out)
			}
			return nil
		}
		for _, a := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "--allow-empty", "-m", seedMessage}} {
			if err := gitIn(env, a...); err != nil {
				return err
			}
		}
		// src2/<name>/ is a second commit one day later: its files are copied over the first
		// ones, and the paths listed in its .remove file are deleted first (SPEC §15).
		second := filepath.Join(caseDir, "src2", e.Name())
		if exists(second) {
			if rm, err := os.ReadFile(filepath.Join(second, ".remove")); err == nil {
				for _, p := range strings.Split(strings.TrimSpace(string(rm)), "\n") {
					if p != "" {
						if err := os.Remove(filepath.Join(dir, p)); err != nil {
							return err
						}
					}
				}
			}
			if err := copyTree(second, dir); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(dir, ".remove")); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			env2 := append(append([]string(nil), s.env...), "GIT_AUTHOR_DATE="+seedDate2, "GIT_COMMITTER_DATE="+seedDate2)
			for _, a := range [][]string{{"add", "-A"}, {"commit", "-q", "--allow-empty", "-m", seedMessage + " 2"}} {
				if err := gitIn(env2, a...); err != nil {
					return err
				}
			}
		}
		s.srcs[e.Name()] = dir
	}
	if !exists(filepath.Join(caseDir, "store")) {
		if err := os.Remove(s.store); err != nil {
			return err
		}
	}
	return nil
}

func (s *sandbox) runStep(dir string, subst map[string]string) ([]string, error) {
	cmdText, err := os.ReadFile(filepath.Join(dir, "cmd"))
	if err != nil {
		return nil, err
	}
	args := strings.Split(string(cmdText), "\n")
	if args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}
	for i := range args {
		args[i] = s.paths(args[i])
	}
	expect := filepath.Join(dir, "expect")
	var problems []string
	// A step's store/ is copied over the store's working tree, without committing, after
	// the paths its .remove file lists are deleted (SPEC §15, steps). The case's own store/
	// is the seed, not an overlay.
	if dir != s.caseDir && exists(filepath.Join(dir, "store")) {
		if err := overlay(filepath.Join(dir, "store"), s.store); err != nil {
			return nil, err
		}
	}
	// A git step runs before the command, in the store, with the runner's environment
	// (SPEC §15, git). An empty line separates commands. Every command must succeed, except
	// that the last one's exit code is compared with expect/git-exit when that file exists.
	if gitText, err := os.ReadFile(filepath.Join(dir, "git")); err == nil {
		cmds := gitCommands(string(gitText))
		wantGitExit := 0
		if b, err := os.ReadFile(filepath.Join(expect, "git-exit")); err == nil {
			if wantGitExit, err = strconv.Atoi(strings.TrimSpace(string(b))); err != nil {
				return nil, fmt.Errorf("expect/git-exit: %v", err)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for i, gitArgs := range cmds {
			for j := range gitArgs {
				gitArgs[j] = s.paths(gitArgs[j])
			}
			if i < len(cmds)-1 || wantGitExit == 0 {
				if _, err := s.git(s.store, gitArgs...); err != nil {
					return nil, err
				}
				continue
			}
			code, stderr, err := s.gitStatus(s.store, gitArgs...)
			if err != nil {
				return nil, err
			}
			if code != wantGitExit {
				problems = append(problems, fmt.Sprintf("git step exit %d, want %d; stderr:\n%s", code, wantGitExit, stderr))
			}
			if b, err := os.ReadFile(filepath.Join(expect, "git-stderr-contains")); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					if line != "" && !pattern(line, subst, false).MatchString(stderr) {
						problems = append(problems, fmt.Sprintf("git step stderr lacks %q; stderr:\n%s", line, stderr))
					}
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	stdin, err := os.ReadFile(filepath.Join(dir, "stdin"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	wantStdout, err := os.ReadFile(filepath.Join(expect, "stdout"))
	if err != nil {
		return nil, err
	}
	exitText, err := os.ReadFile(filepath.Join(expect, "exit"))
	if err != nil {
		return nil, err
	}
	wantExit, err := strconv.Atoi(strings.TrimSpace(string(exitText)))
	if err != nil {
		return nil, fmt.Errorf("expect/exit: %v", err)
	}

	res := s.run(args, stdin)
	if res.err != nil {
		return nil, fmt.Errorf("running memex: %v", res.err)
	}
	if res.exit != wantExit {
		problems = append(problems, fmt.Sprintf("exit %d, want %d; stderr:\n%s", res.exit, wantExit, res.stderr))
	}
	if !pattern(string(wantStdout), subst, true).MatchString(res.stdout) {
		problems = append(problems, fmt.Sprintf("stdout differs\n--- want\n%s--- got\n%s---", wantStdout, res.stdout))
	}
	if b, err := os.ReadFile(filepath.Join(expect, "stderr-contains")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if line != "" && !pattern(line, subst, false).MatchString(res.stderr) {
				problems = append(problems, fmt.Sprintf("stderr lacks %q; stderr:\n%s", line, res.stderr))
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if exists(filepath.Join(expect, "store")) {
		ps, err := compareTrees(filepath.Join(expect, "store"), s.store)
		if err != nil {
			return nil, err
		}
		problems = append(problems, ps...)
	}
	if b, err := os.ReadFile(filepath.Join(expect, "commits")); err == nil {
		got, err := s.commits()
		if err != nil {
			return nil, err
		}
		if got != string(b) {
			problems = append(problems, fmt.Sprintf("commits differ\n--- want\n%s--- got\n%s---", b, got))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return problems, nil
}

// gitCommands splits a git step file into commands: one argument per line, an empty line
// between commands.
func gitCommands(text string) [][]string {
	var cmds [][]string
	var cur []string
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == "" {
			if cur != nil {
				cmds = append(cmds, cur)
			}
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	if cur != nil {
		cmds = append(cmds, cur)
	}
	return cmds
}

// gitStatus runs git like git, but returns its exit code and standard error instead of
// failing, for a git step whose failure is expected.
func (s *sandbox) gitStatus(dir string, args ...string) (int, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Path = filepath.Join(s.bin, "git")
	cmd.Dir = dir
	cmd.Env = s.env
	var errb bytes.Buffer
	cmd.Stderr = &errb
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, errb.String(), nil
	case errors.As(err, &exitErr) && exitErr.ExitCode() >= 0:
		return exitErr.ExitCode(), errb.String(), nil
	}
	return 0, "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
}

// overlay deletes the paths listed in src/.remove from dst, then copies the rest of src
// over dst.
func overlay(src, dst string) error {
	if rm, err := os.ReadFile(filepath.Join(src, ".remove")); err == nil {
		for _, p := range strings.Split(strings.TrimSpace(string(rm)), "\n") {
			if p != "" {
				if err := os.RemoveAll(filepath.Join(dst, p)); err != nil {
					return err
				}
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		return err
	}
	if exists(filepath.Join(src, ".remove")) {
		return os.Remove(filepath.Join(dst, ".remove"))
	}
	return nil
}

// commits is `git log --format=%s` of the store without the seed commit, one subject per
// line, or "" when the store has no commits.
func (s *sandbox) commits() (string, error) {
	if !exists(filepath.Join(s.store, ".git")) {
		return "", nil
	}
	if _, err := s.git(s.store, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return "", nil
	}
	rng := "HEAD"
	if s.seed != "" {
		rng = s.seed + "..HEAD"
	}
	out, err := s.git(s.store, "log", "--format=%s", rng)
	if err != nil || out == "" {
		return "", err
	}
	return out + "\n", nil
}

var placeholder = regexp.MustCompile(`<(SHA7|TIME|STORE|HOME|VERSION)>`)

const rfc3339 = `[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})`

// pattern turns expected text into a regexp: <SHA7> and <TIME> match their shapes, and
// <STORE>, <HOME> and <VERSION> their literal values (SPEC §15). Whole anchors the match.
func pattern(text string, subst map[string]string, whole bool) *regexp.Regexp {
	var b strings.Builder
	if whole {
		b.WriteString(`\A`)
	}
	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(text, -1) {
		b.WriteString(regexp.QuoteMeta(text[last:m[0]]))
		switch name := text[m[2]:m[3]]; name {
		case "SHA7":
			b.WriteString(`[0-9a-f]{7}`)
		case "TIME":
			b.WriteString(rfc3339)
		default:
			b.WriteString(regexp.QuoteMeta(subst[name]))
		}
		last = m[1]
	}
	b.WriteString(regexp.QuoteMeta(text[last:]))
	if whole {
		b.WriteString(`\z`)
	}
	return regexp.MustCompile(b.String())
}

type fileInfo struct {
	data []byte
	exec bool
}

// treeFiles reads every regular file under root, keyed by slash path, skipping .git and
// .memex at the top. Directories are not recorded (SPEC §15, expect/store/).
func treeFiles(root string, skipMeta bool) (map[string]fileInfo, error) {
	files := map[string]fileInfo{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipMeta && (rel == ".git" || rel == ".memex") {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s: not a regular file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = fileInfo{data, info.Mode()&0o111 != 0}
		return nil
	})
	return files, err
}

func compareTrees(want, got string) ([]string, error) {
	w, err := treeFiles(want, false)
	if err != nil {
		return nil, err
	}
	g, err := treeFiles(got, true)
	if err != nil {
		return nil, err
	}
	var names []string
	for n := range w {
		names = append(names, n)
	}
	for n := range g {
		if _, ok := w[n]; !ok {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	var problems []string
	for _, n := range names {
		wf, inWant := w[n]
		gf, inGot := g[n]
		switch {
		case !inGot:
			problems = append(problems, "store: missing "+n)
		case !inWant:
			problems = append(problems, "store: unexpected "+n)
		case !bytes.Equal(wf.data, gf.data):
			problems = append(problems, fmt.Sprintf("store: %s differs\n--- want\n%s--- got\n%s---", n, wf.data, gf.data))
		case wf.exec != gf.exec:
			problems = append(problems, fmt.Sprintf("store: %s executable bit is %v, want %v", n, gf.exec, wf.exec))
		}
	}
	return problems, nil
}

// copyTree copies the regular files, directories and symbolic links under src into dst,
// keeping the executable bit and link targets as written.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s: not a regular file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return err
		}
		return os.Chmod(target, mode) // an overlay may replace a file of another mode

	})
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
