package main

// The SPEC §16.9 guarantees: every command and topic has a page, `help topics` lists exactly
// them, each page's Options table matches `help --json`, every documented option is accepted
// and an undocumented one refused, every ```example block runs against the help fixture and
// matches stream by stream, command pages and the overview have the §16.4 and §16.5 structure,
// exit-code tables match, `COMMAND --help` prints the page, the size limits hold, and
// `help --all` is the concatenation of §16.2.

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
	Name       string          `json:"name"`
	Value      *string         `json:"value"`
	Repeatable bool            `json:"repeatable"`
	Required   json.RawMessage `json:"required"`
	Default    *string         `json:"default"`
}

type ifaceCommand struct {
	Name        string        `json:"name"`
	Summary     string        `json:"summary"`
	Usage       string        `json:"usage"`
	Options     []ifaceOption `json:"options"`
	Positionals []struct {
		Name string `json:"name"`
	} `json:"positionals"`
	ExitCodes []int `json:"exit_codes"`
}

type iface struct {
	GlobalOptions []ifaceOption  `json:"global_options"`
	Commands      []ifaceCommand `json:"commands"`
	Topics        []struct {
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
	if ov, ok := pages["overview"]; !ok {
		fail("help/overview.md is missing")
	} else {
		problems = append(problems, checkOverview(ov, in)...)
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
		problems = append(problems, checkCommandStructure("cmd-"+c.Name, page)...)
		if got := exitTable(section(page, "Exit codes")); fmt.Sprint(got) != fmt.Sprint(c.ExitCodes) {
			fail("help/cmd-%s.md: Exit codes table lists %v, help --json says %v", c.Name, got, c.ExitCodes)
		}
		// `memex COMMAND --help` prints the page (§16.2).
		if r := s.run([]string{c.Name, "--help"}, nil); r.stdout != page || r.exit != 0 {
			fail("help/cmd-%s.md: `memex %s --help` does not print the page (exit %d)", c.Name, c.Name, r.exit)
		}
		if usage := fencedBlock(section(page, "Usage")); usage != c.Usage+"\n" {
			fail("help/cmd-%s.md: Usage block differs from the synopsis\n got: %q\nwant: %q", c.Name, usage, c.Usage+"\n")
		}
		documented := optionsTable(section(page, "Options"))
		accepted := map[string]bool{}
		for _, o := range c.Options {
			accepted[o.Name] = true
			row, ok := documented[o.Name]
			if !ok {
				fail("help/cmd-%s.md: option %s is accepted but not in the Options table", c.Name, o.Name)
				continue
			}
			for _, p := range compareOption(o, row) {
				fail("help/cmd-%s.md: option %s: %s", c.Name, o.Name, p)
			}
		}
		for _, g := range in.GlobalOptions {
			accepted[g.Name] = true // global options are documented for every command (§16.7)
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
		withGlobals := c
		withGlobals.Options = append(append([]ifaceOption{}, c.Options...), in.GlobalOptions...)
		ps, err := tryOptions(memex, git, fixtureDir, withGlobals)
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
// Expected output is standard output, then standard error with each line prefixed "! ", then
// `exit N` when N is not 0; the streams are compared separately.
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
	// A page's direct edits are copied over the working tree, uncommitted (§16.5).
	if edits := filepath.Join(fixtureDir, "edits", page); exists(edits) {
		if err := copyTree(edits, s.store); err != nil {
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
		var stderr strings.Builder
		for _, line := range strings.SplitAfter(r.stderr, "\n") {
			if line != "" {
				stderr.WriteString("! " + line)
			}
		}
		got := r.stdout + stderr.String()
		if got != "" && !strings.HasSuffix(got, "\n") {
			got += "\n"
		}
		if r.exit != 0 {
			got += "exit " + strconv.Itoa(r.exit) + "\n"
		}
		want := ex.output
		if want != "" && !strings.HasSuffix(want, "\n") {
			want += "\n"
		}
		wantOut, wantErr, wantExit := splitExample(want)
		gotOut, gotErr, gotExit := splitExample(got)
		if !pattern(wantOut, subst, true).MatchString(gotOut) || !pattern(wantErr, subst, true).MatchString(gotErr) || wantExit != gotExit {
			problems = append(problems, fmt.Sprintf("help/%s.md: example `%s`\n--- got ---\n%s--- want ---\n%s", page, ex.command, got, want))
		}
	}
	return problems, nil
}

// splitExample splits an example's expected output (§16.5) into its standard output lines,
// its standard error lines (each written with a "! " prefix, returned without it) and its
// final `exit N` line, so that each stream is compared on its own.
func splitExample(text string) (stdout, stderr, exit string) {
	lines := strings.SplitAfter(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if n := len(lines); n > 0 && strings.HasPrefix(lines[n-1], "exit ") {
		exit = lines[n-1]
		lines = lines[:n-1]
	}
	var out, errs strings.Builder
	for _, l := range lines {
		if rest, ok := strings.CutPrefix(l, "! "); ok {
			errs.WriteString(rest)
		} else if errs.Len() == 0 {
			out.WriteString(l)
		} else {
			errs.WriteString("\x00" + l) // a stdout line after stderr never matches
		}
	}
	return out.String(), errs.String(), exit
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

// section returns the text of the `## <title>` section of a page, up to the next `## `
// heading. Lines inside fenced code blocks are never headings.
func section(page, title string) string {
	var b strings.Builder
	in := false
	for _, h := range splitSections(page) {
		if h.title == title && !in {
			b.WriteString(h.body)
			in = true
		}
	}
	return b.String()
}

type pageSection struct{ title, body string }

// splitSections splits a page at its `## ` headings outside fenced code. The text before the
// first heading has the title "".
func splitSections(page string) []pageSection {
	out := []pageSection{{}}
	inFence := false
	lines := strings.SplitAfter(page, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			out = append(out, pageSection{title: strings.TrimSpace(strings.TrimPrefix(line, "## "))})
			continue
		}
		out[len(out)-1].body += line
	}
	return out
}

// commandSections is the §16.5 section order; the first four marked required must be present.
var commandSections = []struct {
	title    string
	required bool
}{
	{"Usage", true}, {"Description", true}, {"Options", false}, {"Output", true},
	{"Exit codes", true}, {"Examples", true}, {"Mistakes to avoid", false}, {"See also", false},
}

// checkCommandStructure checks §16.5: section order, Description at most 15 lines, two to
// five examples.
func checkCommandStructure(name, page string) []string {
	var out []string
	secs := splitSections(page)
	next := 0
	seen := map[string]bool{}
	for _, s := range secs[1:] {
		i := next
		for i < len(commandSections) && commandSections[i].title != s.title {
			i++
		}
		if i == len(commandSections) {
			out = append(out, fmt.Sprintf("help/%s.md: section %q is not a §16.5 section or is out of order", name, s.title))
			continue
		}
		seen[s.title] = true
		next = i + 1
	}
	for _, c := range commandSections {
		if c.required && !seen[c.title] {
			out = append(out, fmt.Sprintf("help/%s.md: missing section %q", name, c.title))
		}
	}
	if n := lineCount(strings.Trim(section(page, "Description"), "\n") + "\n"); n > 15 {
		out = append(out, fmt.Sprintf("help/%s.md: Description is %d lines; the limit is 15", name, n))
	}
	if n := len(exampleBlocks(page)); n < 2 || n > 5 {
		out = append(out, fmt.Sprintf("help/%s.md: %d examples; a command page has two to five", name, n))
	}
	return out
}

var exitRow = regexp.MustCompile(`^\| *([0-9]+) *\|`)

// exitTable returns the codes of an Exit codes table, in order.
func exitTable(text string) []int {
	var out []int
	for _, line := range strings.Split(text, "\n") {
		if m := exitRow.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			out = append(out, n)
		}
	}
	return out
}

// checkOverview checks §16.4's order: the version line, the agent line, the model, the
// everyday loop, the command table (every command and its summary, in order), the topic
// table, and `memex help topics` as where to go next.
func checkOverview(page string, in iface) []string {
	var out []string
	lines := strings.Split(page, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "# memex ") || !strings.HasPrefix(lines[2], "If you are an AI agent: run `memex help agents` now") {
		out = append(out, "help/overview.md: must start with the version line, a blank line and the agent line (§16.4)")
	}
	var titles []string
	secs := splitSections(page)
	for _, s := range secs[1:] {
		titles = append(titles, s.title)
	}
	if want := []string{"The model", "The everyday loop", "Commands", "Topics"}; strings.Join(titles, "|") != strings.Join(want, "|") {
		out = append(out, fmt.Sprintf("help/overview.md: sections %q, want %q (§16.4)", titles, want))
	}
	var rows, want []string
	for _, line := range strings.Split(section(page, "Commands"), "\n") {
		if m := optionCell.FindStringSubmatch(line); m == nil && strings.HasPrefix(line, "| `") {
			rows = append(rows, line)
		}
	}
	for _, c := range in.Commands {
		want = append(want, "| `"+c.Name+"` | "+c.Summary+" |")
	}
	if strings.Join(rows, "\n") != strings.Join(want, "\n") {
		out = append(out, "help/overview.md: the command table must list every command with its summary, in order (§16.4)")
	}
	if last := secs[len(secs)-1].body; !strings.Contains(last, "`memex help topics`") {
		out = append(out, "help/overview.md: must end by pointing to `memex help topics` (§16.4)")
	}
	return out
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

// optionRow is one row of an Options table: option, value, default, meaning (§16.5).
type optionRow struct{ value, def, meaning string }

// optionsTable returns the rows of an Options table, by option name.
func optionsTable(text string) map[string]optionRow {
	out := map[string]optionRow{}
	for _, line := range strings.Split(text, "\n") {
		m := optionCell.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for len(cells) < 4 {
			cells = append(cells, "")
		}
		clean := func(c string) string { return strings.ReplaceAll(strings.TrimSpace(c), "`", "") }
		out[m[1]] = optionRow{value: clean(cells[1]), def: clean(cells[2]), meaning: clean(strings.Join(cells[3:], "|"))}
	}
	return out
}

// compareOption checks a table row against help --json (§16.5): the value and default cells
// match (empty for none), the meaning says "repeatable" exactly when the option is, and it
// says "(required)" or "(required <condition>)" exactly when the option is required.
func compareOption(o ifaceOption, row optionRow) []string {
	var out []string
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	if row.value != str(o.Value) {
		out = append(out, fmt.Sprintf("value %q, help --json says %q", row.value, str(o.Value)))
	}
	if row.def != str(o.Default) {
		out = append(out, fmt.Sprintf("default %q, help --json says %q", row.def, str(o.Default)))
	}
	if strings.Contains(row.meaning, "repeatable") != o.Repeatable {
		out = append(out, fmt.Sprintf("meaning %q must say \"repeatable\" exactly when help --json does (%v)", row.meaning, o.Repeatable))
	}
	want := ""
	var cond string
	switch {
	case string(o.Required) == "true":
		want = "(required)"
	case json.Unmarshal(o.Required, &cond) == nil:
		want = "(required " + cond + ")"
	}
	if has := strings.Contains(row.meaning, "(required"); has != (want != "") || (want != "" && !strings.Contains(row.meaning, want)) {
		out = append(out, fmt.Sprintf("meaning %q must say %q exactly when help --json requires it", row.meaning, want))
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
