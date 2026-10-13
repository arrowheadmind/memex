// Package importv3 converts brainctl v3 brains into a memex store (SPEC §13).
package importv3

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arrowheadmind/memex/internal/gitx"
)

// Source is one brain, read through git at a fixed revision (SPEC §13.1).
type Source struct {
	Path     string // as given on the command line
	Abs      string // absolute path of the repository
	Rev      string // the revision as given, HEAD by default
	Commit   string // the resolved commit
	Basename string
	Primary  bool
	git      *gitx.Git
	mu       sync.Mutex        // guards the caches below; reads may run concurrently
	files    []string          // every tracked path at Rev, sorted
	fileSet  map[string]bool   // files as a set
	content  map[string]string // path -> text, cached
	blames   map[string][]string
	firstDay map[string]string
	lastDay  map[string]string
}

// ParseSourceArg splits SRC[@REV].
func ParseSourceArg(arg string) (string, string) {
	if i := strings.LastIndex(arg, "@"); i > 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

// OpenSource resolves the repository and its revision. Without a revision the working tree
// must be clean (the import must describe a commit).
func OpenSource(arg string, primary bool, timeout time.Duration) (*Source, error) {
	p, rev := ParseSourceArg(arg)
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	g := gitx.New(abs, timeout, gitx.Extra("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"))
	if _, err := run(g, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("%s is not a git repository", p)
	}
	if rev == "" {
		status, err := run(g, "status", "--porcelain")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(status) != "" {
			return nil, fmt.Errorf("%s has uncommitted changes; commit them or give SRC@REV", p)
		}
		rev = "HEAD"
	}
	commit, err := run(g, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%s: no revision %q", p, rev)
	}
	commit = strings.TrimSpace(commit)
	list, err := run(g, "ls-tree", "-r", "--name-only", "-z", commit)
	if err != nil {
		return nil, err
	}
	s := &Source{
		Path: p, Abs: abs, Rev: rev, Commit: commit, Basename: filepath.Base(abs), Primary: primary,
		git: g, fileSet: map[string]bool{}, content: map[string]string{},
		blames: map[string][]string{}, firstDay: map[string]string{}, lastDay: map[string]string{},
	}
	for _, f := range strings.Split(list, "\x00") {
		if f != "" {
			s.files = append(s.files, f)
			s.fileSet[f] = true
		}
	}
	sort.Strings(s.files)
	return s, nil
}

// gitConfig isolates every source read from the operator's git configuration, so that two
// machines read the same history the same way (SPEC §13.1).
var gitConfig = []string{"-c", "blame.ignoreRevsFile=", "-c", "log.showSignature=false", "-c", "diff.renames=true"}

func run(g *gitx.Git, args ...string) (string, error) {
	return g.Run(append(append([]string(nil), gitConfig...), args...)...)
}

func (s *Source) run(args ...string) (string, error) { return run(s.git, args...) }

// LineRangeDay dates a line unit (SPEC §13.3): the earlier of
//   - the commit in `git log -L<a>,<b>:<path>` that created the range (the newest commit
//     whose hunk has an empty old side), or the oldest commit of that log when none does;
//     this follows the lines through edits, so an edited row keeps its date;
//   - the oldest commit of `git log -S<text>` for the unit's first line, which a move inside
//     the file does not touch, so a moved backlog item keeps its date.
func (s *Source) LineRangeDay(p string, a, b int, firstLine string) (string, error) {
	out, err := s.run("log", fmt.Sprintf("-L%d,%d:%s", a, b, p), "--format=%x00%aI", s.Commit)
	if err != nil {
		return "", err
	}
	byL := ""
	for _, block := range strings.Split(out, "\x00")[1:] {
		date := block[:10]
		byL = date
		if created := hunkCreates.MatchString(block); created {
			break
		}
	}
	byS := ""
	if strings.TrimSpace(firstLine) != "" {
		out, err := s.run("log", "-S"+firstLine, "--format=%aI", s.Commit, "--", p)
		if err != nil {
			return "", err
		}
		lines := nonEmptyLines(out)
		if len(lines) > 0 {
			byS = lines[len(lines)-1][:10]
		}
	}
	switch {
	case byL == "" && byS == "":
		return s.FirstDay(p)
	case byL == "" || (byS != "" && byS < byL):
		return byS, nil
	}
	return byL, nil
}

// hunkCreates matches a -L hunk whose old side is empty: the lines did not exist before.
var hunkCreates = regexp.MustCompile(`(?m)^@@ -\d+,0 \+\d+(,\d+)? @@`)

var dateLine = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)

// Files returns every tracked path at the revision, sorted.
func (s *Source) Files() []string { return s.files }

// Has reports whether the path is tracked at the revision.
func (s *Source) Has(p string) bool { return s.fileSet[p] }

// Dir lists the tracked files directly or indirectly under dir, sorted.
func (s *Source) Dir(dir string) []string {
	var out []string
	prefix := strings.TrimSuffix(dir, "/") + "/"
	for _, f := range s.files {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// Read returns a file's text at the revision.
func (s *Source) Read(p string) (string, error) {
	s.mu.Lock()
	c, ok := s.content[p]
	s.mu.Unlock()
	if ok {
		return c, nil
	}
	out, err := s.run("show", s.Commit+":"+p)
	if err != nil {
		return "", fmt.Errorf("%s: cannot read %s at %s", s.Path, p, s.Rev)
	}
	s.mu.Lock()
	s.content[p] = out
	s.mu.Unlock()
	return out, nil
}

// ReadBytes returns a file's bytes at the revision (attachments).
func (s *Source) ReadBytes(p string) ([]byte, error) {
	out, err := s.Read(p)
	return []byte(out), err
}

// FirstDay is the YYYY-MM-DD part of the author time of the earliest commit touching the
// file, following renames.
func (s *Source) FirstDay(p string) (string, error) {
	s.mu.Lock()
	d, ok := s.firstDay[p]
	s.mu.Unlock()
	if ok {
		return d, nil
	}
	out, err := s.run("log", "--follow", "--format=%aI", s.Commit, "--", p)
	if err != nil {
		return "", err
	}
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("%s: no history for %s", s.Path, p)
	}
	d = lines[len(lines)-1][:10]
	s.mu.Lock()
	s.firstDay[p] = d
	s.mu.Unlock()
	return d, nil
}

// LastDay is the date of the last commit touching the file.
func (s *Source) LastDay(p string) (string, error) {
	s.mu.Lock()
	d, ok := s.lastDay[p]
	s.mu.Unlock()
	if ok {
		return d, nil
	}
	out, err := s.run("log", "-1", "--format=%aI", s.Commit, "--", p)
	if err != nil {
		return "", err
	}
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("%s: no history for %s", s.Path, p)
	}
	d = lines[0][:10]
	s.mu.Lock()
	s.lastDay[p] = d
	s.mu.Unlock()
	return d, nil
}

// CommitDay is the author date of the source revision itself.
func (s *Source) CommitDay() (string, error) {
	out, err := s.run("log", "-1", "--format=%aI", s.Commit)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out)[:10], nil
}

var blameTime = regexp.MustCompile(`(?m)^author-time (\d+)$`)
var blameTZ = regexp.MustCompile(`(?m)^author-tz ([+-]\d{4})$`)

// BlameDays returns, per line (1-based index into the slice; index 0 unused), the author
// date of the commit that last touched the line.
func (s *Source) BlameDays(p string) ([]string, error) {
	if b, ok := s.blames[p]; ok {
		return b, nil
	}
	out, err := s.run("blame", "--line-porcelain", s.Commit, "--", p)
	if err != nil {
		return nil, err
	}
	// Each line's record starts with "<sha> <orig> <final> [<count>]" and holds
	// author-time/author-tz only for the first line of each group; later lines of the same
	// commit group omit the header fields, so remember them per commit.
	var days []string
	days = append(days, "")
	type info struct{ t, tz string }
	byCommit := map[string]info{}
	var cur string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case isBlameHeader(line):
			cur = line[:strings.IndexByte(line, ' ')]
			fields := strings.Fields(line)
			// final line number is fields[2]; records come in order, one per line.
			_ = fields
			days = append(days, "") // placeholder; filled when the tab line arrives
		case strings.HasPrefix(line, "author-time "):
			in := byCommit[cur]
			in.t = strings.TrimPrefix(line, "author-time ")
			byCommit[cur] = in
		case strings.HasPrefix(line, "author-tz "):
			in := byCommit[cur]
			in.tz = strings.TrimPrefix(line, "author-tz ")
			byCommit[cur] = in
		case strings.HasPrefix(line, "\t"):
			in := byCommit[cur]
			days[len(days)-1] = dayOf(in.t, in.tz)
		}
	}
	s.blames[p] = days
	return days, nil
}

// isBlameHeader recognizes "<sha> <orig> <final> [<count>]" with a 40- or 64-hex hash.
func isBlameHeader(line string) bool {
	sp := strings.IndexByte(line, ' ')
	return (sp == 40 || sp == 64) && isHex(line[:sp])
}

func isHex(s string) bool {
	for _, c := range s {
		if !(('0' <= c && c <= '9') || ('a' <= c && c <= 'f')) {
			return false
		}
	}
	return true
}

// dayOf converts an epoch and a +HHMM offset to the date in that offset.
func dayOf(epoch, tz string) string {
	secs, err := strconv.ParseInt(epoch, 10, 64)
	if err != nil {
		return ""
	}
	off := 0
	if len(tz) == 5 {
		h, _ := strconv.Atoi(tz[1:3])
		m, _ := strconv.Atoi(tz[3:5])
		off = h*3600 + m*60
		if tz[0] == '-' {
			off = -off
		}
	}
	return time.Unix(secs, 0).UTC().Add(time.Duration(off) * time.Second).Format(time.DateOnly)
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// Prefixes returns the path prefixes §13.5 strips for this source: the path as given, with
// $HOME as "~", the basename followed by "/", and "./".
func (s *Source) Prefixes(home string) []string {
	set := map[string]bool{}
	add := func(p string) {
		if p != "" {
			set[strings.TrimSuffix(p, "/")+"/"] = true
		}
	}
	add(s.Path)
	add(s.Abs)
	if home != "" && strings.HasPrefix(s.Abs, home+"/") {
		add("~/" + strings.TrimPrefix(s.Abs, home+"/"))
	}
	add(s.Basename)
	add(".")
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	// longest first, so that a longer prefix wins over its own suffixes
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// stem returns the filename without its extension.
func stem(p string) string {
	b := path.Base(p)
	return strings.TrimSuffix(b, path.Ext(b))
}
