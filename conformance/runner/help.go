package main

// The SPEC §16.9 guarantees: every command and topic has a page, `help topics` lists exactly
// them, each page's Options table matches `help --json`, every documented option is accepted
// and an undocumented one refused, every ```example block runs against the help fixture and
// matches, the size limits hold, and `help --all` is the concatenation of §16.2.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ifaceOption struct {
	Name     string          `json:"name"`
	Value    *string         `json:"value"`
	Required json.RawMessage `json:"required"`
}

type ifaceCommand struct {
	Name        string        `json:"name"`
	Summary     string        `json:"summary"`
	Usage       string        `json:"usage"`
	Options     []ifaceOption `json:"options"`
	Positionals []struct {
		Name string `json:"name"`
	} `json:"positionals"`
}

type iface struct {
	Commands []ifaceCommand `json:"commands"`
	Topics   []struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
	} `json:"topics"`
}

const (
	overviewMaxLines = 80
	commandMaxLines  = 120
	topicMaxLines    = 200
	agentsMaxLines   = 60
	allMaxLines      = 3000
	maxLineLength    = 100
)

// runHelpChecks performs the §16.9 checks. It returns one problem string per failed check.
func runHelpChecks(memex, git, version, pagesDir, fixtureDir string) ([]string, error) {
	var problems []string
	fail := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	s, err := newSandbox(memex, git, false)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(s.root)

	r := s.run([]string{"help", "--json"}, nil)
	if r.err != nil || r.exit != 0 {
		return nil, fmt.Errorf("help --json: exit %d %v\n%s", r.exit, r.err, r.stderr)
	}
	var in iface
	if err := json.Unmarshal([]byte(r.stdout), &in); err != nil {
		return nil, fmt.Errorf("help --json: %v", err)
	}

	pages := map[string]string{}
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(pagesDir, e.Name()))
		if err != nil {
			return nil, err
		}
		pages[strings.TrimSuffix(e.Name(), ".md")] = string(b)
	}

	// Every command and topic has a page, and nothing else is a page (§16.9).
	expected := map[string]bool{"overview": true}
	var topicLines []string
	for _, c := range in.Commands {
		expected["cmd-"+c.Name] = true
		if _, ok := pages["cmd-"+c.Name]; !ok {
			fail("command %s has no help page help/cmd-%s.md", c.Name, c.Name)
		}
		topicLines = append(topicLines, c.Name+"  "+c.Summary)
	}
	for _, t := range in.Topics {
		expected["topic-"+t.Name] = true
		if _, ok := pages["topic-"+t.Name]; !ok {
			fail("topic %s has no help page help/topic-%s.md", t.Name, t.Name)
		}
		topicLines = append(topicLines, t.Name+"  "+t.Summary)
	}
	for name := range pages {
		if !expected[name] {
			fail("help/%s.md is not a page of any command or topic", name)
		}
	}
	if _, ok := pages["overview"]; !ok {
		fail("help/overview.md is missing")
	}

	// `help topics` lists exactly them (§16.2, §16.9).
	r = s.run([]string{"help", "topics"}, nil)
	if want := strings.Join(topicLines, "\n") + "\n"; r.stdout != want || r.exit != 0 {
		fail("help topics: got\n%s\nwant\n%s", r.stdout, want)
	}

	// `help --all` is the concatenation of §16.2, and `help` is the overview.
	r = s.run([]string{"help"}, nil)
	overview := r.stdout
	if wantFirst := "# memex " + version + " (store format 1)"; !strings.HasPrefix(overview, wantFirst+"\n") {
		fail("help: first line %q, want %q", firstLine(overview), wantFirst)
	}
	if ov, ok := pages["overview"]; ok {
		_, body, _ := strings.Cut(ov, "\n")
		if _, got, _ := strings.Cut(overview, "\n"); got != body {
			fail("help: output differs from help/overview.md after the version line")
		}
	}
	var all strings.Builder
	all.WriteString(overview)
	for _, t := range in.Topics {
		all.WriteString("---\n")
		all.WriteString(pages["topic-"+t.Name])
	}
	for _, c := range in.Commands {
		all.WriteString("---\n")
		all.WriteString(pages["cmd-"+c.Name])
	}
	r = s.run([]string{"help", "--all"}, nil)
	if r.stdout != all.String() {
		fail("help --all is not the overview plus every topic and command page separated by ---")
	}
	if n := lineCount(r.stdout); n >= allMaxLines {
		fail("help --all is %d lines; it must stay under %d", n, allMaxLines)
	}

	// Size and format limits (§16.3–16.6).
	for name, text := range pages {
		limit := topicMaxLines
		switch {
		case name == "overview":
			limit = overviewMaxLines
		case name == "topic-agents":
			limit = agentsMaxLines
		case strings.HasPrefix(name, "cmd-"):
			limit = commandMaxLines
		}
		if n := lineCount(text); n > limit {
			fail("help/%s.md is %d lines; the limit is %d", name, n, limit)
		}
		for i, line := range longLines(text) {
			_ = i
			fail("help/%s.md: line longer than %d characters: %q", name, maxLineLength, line)
		}
		if strings.Contains(text, "\x1b[") {
			fail("help/%s.md contains an ANSI escape", name)
		}
	}

	// Command pages: usage, options and positionals match the interface (§16.5, §16.9).
	for _, c := range in.Commands {
		page, ok := pages["cmd-"+c.Name]
		if !ok {
			continue
		}
		if !strings.HasPrefix(page, "# memex "+c.Name+"\n") {
			fail("help/cmd-%s.md: first line must be `# memex %s`", c.Name, c.Name)
		}
		if usage := fencedBlock(section(page, "Usage")); usage != c.Usage+"\n" {
			fail("help/cmd-%s.md: Usage block differs from the synopsis\n got: %q\nwant: %q", c.Name, usage, c.Usage+"\n")
		}
		documented := optionsTable(section(page, "Options"))
		accepted := map[string]bool{}
		for _, o := range c.Options {
			accepted[o.Name] = true
			if !documented[o.Name] {
				fail("help/cmd-%s.md: option %s is accepted but not in the Options table", c.Name, o.Name)
			}
		}
		for name := range documented {
			if !accepted[name] {
				fail("help/cmd-%s.md: option %s is in the Options table but not accepted", c.Name, name)
			}
		}
		if len(c.Options) == 0 && section(page, "Options") != "" {
			fail("help/cmd-%s.md: has an Options section but accepts no options", c.Name)
		}
		for _, p := range c.Positionals {
			if !strings.Contains(c.Usage, p.Name) {
				fail("help/cmd-%s.md: positional %s is not in the synopsis", c.Name, p.Name)
			}
		}
	}

	// Every documented option is accepted; an undocumented one is refused (§16.9). Each
	// command runs in a store seeded from the fixture, so that a mutating command may do
	// whatever it does; only the "unknown option" refusal is observed.
	for _, c := range in.Commands {
		ps, err := tryOptions(memex, git, fixtureDir, c)
		if err != nil {
			return problems, err
		}
		problems = append(problems, ps...)
	}

	// Every example runs and matches (§16.9).
	for name, text := range pages {
		examples := exampleBlocks(text)
		if len(examples) == 0 {
			continue
		}
		ps, err := runExamples(memex, git, version, fixtureDir, name, examples)
		if err != nil {
			return problems, fmt.Errorf("help/%s.md: %v", name, err)
		}
		problems = append(problems, ps...)
	}
	return problems, nil
}

// tryOptions runs the command once per documented option, with a value when it takes one,
// and once with an option that does not exist.
func tryOptions(memex, git, fixtureDir string, c ifaceCommand) ([]string, error) {
	var problems []string
	s, err := newSandbox(memex, git, false)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(s.root)
	if err := s.seedStore(filepath.Join(fixtureDir, "store"), false); err != nil {
		return nil, err
	}
	positionals := minimalPositionals(c)
	for _, o := range c.Options {
		args := append([]string{c.Name}, positionals...)
		if o.Value != nil {
			args = append(args, o.Name+"="+sampleValue(o.Name))
		} else {
			args = append(args, o.Name)
		}
		r := s.run(args, nil)
		if r.err != nil {
			return nil, r.err
		}
		if strings.Contains(r.stderr, "unknown option "+o.Name) {
			problems = append(problems, fmt.Sprintf("%s: documented option %s is refused as unknown", c.Name, o.Name))
		}
	}
	args := append([]string{c.Name}, positionals...)
	args = append(args, "--no-such-option-xyz")
	r := s.run(args, nil)
	if r.err != nil {
		return nil, r.err
	}
	if r.exit != 2 || !strings.Contains(r.stderr, "unknown option --no-such-option-xyz") {
		problems = append(problems, fmt.Sprintf("%s: an undocumented option must exit 2 as unknown; got exit %d, stderr %q", c.Name, r.exit, r.stderr))
	}
	return problems, nil
}

// minimalPositionals gives a command the positionals it needs so that option parsing is
// reached: their values do not have to make sense.
func minimalPositionals(c ifaceCommand) []string {
	var out []string
	for _, p := range c.Positionals {
		switch p.Name {
		case "TYPE":
			out = append(out, "rule")
		case "brain-v3":
			out = append(out, "brain-v3")
		case "ASSIGNMENT":
			out = append(out, "summary=x")
		case "COMMAND|TOPIC":
		default:
			out = append(out, "x")
		}
	}
	return out
}

func sampleValue(option string) string {
	switch option {
	case "--limit", "--review-days", "--max-chars":
		return "5"
	case "--verified":
		return "2026-01-01"
	case "--body":
		return "-"
	case "--fields":
		return "id"
	case "--type", "--to":
		return "rule"
	case "--status":
		return "active"
	}
	return "x"
}

// example is one ```example block: the command line after "$ " and the expected output.
type example struct {
	command string
	output  string
}

var exampleStart = regexp.MustCompile("(?m)^```example\n")

// exampleBlocks extracts every ```example block of a page (§16.5).
func exampleBlocks(page string) []example {
	var out []example
	for _, m := range exampleStart.FindAllStringIndex(page, -1) {
		rest := page[m[1]:]
		end := strings.Index(rest, "\n```\n")
		if end < 0 {
			if strings.HasSuffix(rest, "\n```") {
				end = len(rest) - 4
			} else {
				continue
			}
		}
		body := rest[:end]
		cmd, output, _ := strings.Cut(body, "\n")
		out = append(out, example{command: strings.TrimPrefix(cmd, "$ "), output: output})
	}
	return out
}

// runExamples runs a page's examples in order in one store seeded from the help fixture,
// with the fixture's home/ copied into HOME, and compares each output with placeholders.
// Expected output is standard output, then standard error, then `exit N` when N is not 0.
func runExamples(memex, git, version, fixtureDir, page string, examples []example) ([]string, error) {
	var problems []string
	s, err := newSandbox(memex, git, false)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(s.root)
	if err := s.seedStore(filepath.Join(fixtureDir, "store"), false); err != nil {
		return nil, err
	}
	if exists(filepath.Join(fixtureDir, "home")) {
		if err := copyTree(filepath.Join(fixtureDir, "home"), s.home); err != nil {
			return nil, err
		}
	}
	subst := map[string]string{"STORE": s.store, "HOME": s.home, "VERSION": version}
	for _, ex := range examples {
		words, err := splitShell(ex.command)
		if err != nil {
			return nil, fmt.Errorf("example %q: %v", ex.command, err)
		}
		if len(words) == 0 || words[0] != "memex" {
			problems = append(problems, fmt.Sprintf("help/%s.md: example %q does not run memex", page, ex.command))
			continue
		}
		for i := range words {
			words[i] = s.paths(words[i])
		}
		r := s.run(words[1:], nil)
		if r.err != nil {
			return nil, r.err
		}
		got := r.stdout + r.stderr
		if r.exit != 0 {
			got += "exit " + strconv.Itoa(r.exit) + "\n"
		}
		want := ex.output
		if want != "" && !strings.HasSuffix(want, "\n") {
			want += "\n"
		}
		if !pattern(want, subst, true).MatchString(got) {
			problems = append(problems, fmt.Sprintf("help/%s.md: example `%s`\n--- got ---\n%s--- want ---\n%s", page, ex.command, got, want))
		}
	}
	return problems, nil
}

// splitShell splits a command line like a POSIX shell without expansions: single quotes are
// literal, double quotes allow \" and \\, and a backslash outside quotes escapes the next
// character. Examples never need more than that.
func splitShell(line string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated single quote")
			}
			cur.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord = true
		case ch == '"':
			i++
			for ; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) && (line[i+1] == '"' || line[i+1] == '\\') {
					i++
				}
				cur.WriteByte(line[i])
			}
			if i >= len(line) {
				return nil, fmt.Errorf("unterminated double quote")
			}
			inWord = true
		case ch == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
			inWord = true
		case ch == ' ' || ch == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(ch)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

// section returns the text of the `## <title>` section of a page, up to the next `## `.
func section(page, title string) string {
	marker := "\n## " + title + "\n"
	start := strings.Index(page, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	rest := page[start:]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end+1]
	}
	return rest
}

// fencedBlock returns the content of the first fenced code block in text.
func fencedBlock(text string) string {
	start := strings.Index(text, "```")
	if start < 0 {
		return ""
	}
	rest := text[start:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return ""
	}
	rest = rest[nl+1:]
	end := strings.Index(rest, "\n```")
	if end < 0 {
		return rest
	}
	return rest[:end+1]
}

var optionCell = regexp.MustCompile("^\\| *`(-[^`]*)`")

// optionsTable returns the option names in the first column of a Markdown table.
func optionsTable(text string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if m := optionCell.FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

func lineCount(text string) int {
	return strings.Count(text, "\n")
}

// longLines returns the lines over the limit, ignoring table rows and fenced code (§16.3).
func longLines(text string) []string {
	var out []string
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(line, "|") {
			continue
		}
		if utf8.RuneCountInString(line) > maxLineLength {
			out = append(out, line)
		}
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
