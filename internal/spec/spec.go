// Package spec is the single definition of memex's interface: commands, options, positionals,
// types, statuses, frontmatter fields and exit codes. The parser, the validator and
// `help --json` are all driven from these tables; nothing else defines them.
//
// Sources: SPEC §5.2 (types), §5.3 (fields), §10 (exit codes, global options), §11 (commands),
// §16.7 (shape).
package spec

import "slices"

// StoreFormat is the store format version this implementation reads and writes (SPEC §4).
const StoreFormat = 1

// Exit codes (SPEC §10).
const (
	ExitOK        = 0
	ExitFailure   = 1
	ExitUsage     = 2
	ExitNoMatch   = 3
	ExitLock      = 4
	ExitNotPushed = 5
	ExitConflict  = 6
	ExitInternal  = 70
)

// ExitCode is one row of the SPEC §10 exit-code table.
type ExitCode struct {
	Code    int
	Meaning string
}

// ExitCodes is the SPEC §10 table, in its order.
var ExitCodes = []ExitCode{
	{ExitOK, "Success"},
	{ExitFailure, "Validation error, check failure, or refused operation"},
	{ExitUsage, "Usage or configuration error"},
	{ExitNoMatch, "Search found nothing visible"},
	{ExitLock, "Lock timeout"},
	{ExitNotPushed, "Committed but not pushed, or sync failed"},
	{ExitConflict, "Rebase conflict"},
	{ExitInternal, "Internal error (a bug)"},
}

// Type is one row of the SPEC §5.2 table. The first status is the default for new entries.
type Type struct {
	Name      string
	Directory string
	Statuses  []string
	Visible   []string
}

// Types is the SPEC §5.2 table, in its order, which is also the ranking order of §8.3.
var Types = []Type{
	{"rule", "rules", []string{"active", "superseded", "retired"}, []string{"active"}},
	{"decision", "decisions", []string{"accepted", "proposed", "superseded", "rejected"}, []string{"accepted", "proposed"}},
	{"runbook", "runbooks", []string{"active", "superseded", "retired"}, []string{"active"}},
	{"reference", "reference", []string{"active", "superseded", "retired"}, []string{"active"}},
	{"task", "tasks", []string{"open", "in-progress", "blocked", "done", "dropped"}, []string{"open", "in-progress", "blocked"}},
	{"plan", "plans", []string{"draft", "active", "done", "abandoned", "superseded"}, []string{"draft", "active"}},
	{"journal", "journal", []string{"raw", "kept", "promoted", "superseded"}, []string{"raw", "kept", "promoted"}},
}

// Field kinds (SPEC §5.1).
const (
	KindString  = "string"
	KindDate    = "date"
	KindInteger = "integer"
	KindBoolean = "boolean"
	KindArray   = "array"
)

// AllTypes is the AllowedOn value for a field allowed on every type.
const AllTypes = "*"

// Field is one row of the SPEC §5.3 table. Required is "yes", "no", or a condition.
type Field struct {
	Name      string
	Kind      string
	Required  string
	AllowedOn []string
}

// Fields is the SPEC §5.3 table, in canonical order.
var Fields = []Field{
	{"title", KindString, "yes", []string{AllTypes}},
	{"status", KindString, "yes", []string{AllTypes}},
	{"created", KindDate, "yes", []string{AllTypes}},
	{"summary", KindString, "no", []string{AllTypes}},
	{"tags", KindArray, "no", []string{AllTypes}},
	{"keywords", KindArray, "no", []string{AllTypes}},
	{"repos", KindArray, "no", []string{AllTypes}},
	{"hosts", KindArray, "no", []string{AllTypes}},
	{"always", KindBoolean, "no", []string{"rule"}},
	{"allow_secret", KindBoolean, "no", []string{AllTypes}},
	{"kind", KindString, "no", []string{"plan"}},
	{"parent", KindString, "no", []string{"plan", "task"}},
	{"sources", KindArray, "no", []string{AllTypes}},
	{"superseded_by", KindString, "when status is superseded", []string{AllTypes}},
	{"verified", KindDate, "yes", []string{"reference"}},
	{"review_days", KindInteger, "no", []string{"reference"}},
	{"aliases", KindArray, "no", []string{AllTypes}},
	{"body_sha256", KindString, "yes", []string{"journal"}},
}

// Option is one command-line option. Value is the placeholder for its value, or "" for a
// flag that takes none. Default is "" when there is none.
type Option struct {
	Name       string
	Value      string
	Repeatable bool
	Required   bool
	Default    string
	Summary    string
}

// Positional is one positional argument.
type Positional struct {
	Name       string
	Repeatable bool
	Required   bool
}

// Command is one memex command. Usage is the synopsis exactly as in SPEC §11, one line per
// form; continuation lines keep their indentation.
type Command struct {
	Name        string
	Summary     string
	Usage       string
	Mutating    bool
	Options     []Option
	Positionals []Positional
	ExitCodes   []int
}

// GlobalUsage is the general form of every invocation (SPEC §10).
const GlobalUsage = "memex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]"

// GlobalOptions are accepted before the command (SPEC §10, §16.2).
var GlobalOptions = []Option{
	{Name: "--store", Value: "PATH", Summary: "Store to use; overrides MEMEX_STORE and the config file"},
	{Name: "--help", Summary: "Print the overview and exit"},
	{Name: "--version", Summary: "Print the version and exit"},
}

// HelpOptions are accepted by every command and print its help page (SPEC §10, §16.2).
var HelpOptions = []string{"--help", "-h"}

var newOptions = []Option{
	{Name: "--status", Value: "S", Default: "first status of the type", Summary: "Initial status"},
	{Name: "--tag", Value: "X", Repeatable: true, Summary: "Tag for filtering"},
	{Name: "--keyword", Value: "K", Repeatable: true, Summary: "Word a reader would search with"},
	{Name: "--repo", Value: "R", Repeatable: true, Default: "the current repository", Summary: "Repository the entry concerns; replaces the current one"},
	{Name: "--no-repo", Summary: "Do not attribute the entry to the current repository"},
	{Name: "--host", Value: "H", Repeatable: true, Summary: "Machine the entry concerns"},
	{Name: "--always", Summary: "Rule printed by every preflight (rules only)"},
	{Name: "--kind", Value: "K", Summary: "prompt or report (plans only)"},
	{Name: "--parent", Value: "ID", Summary: "Enclosing plan (plans and tasks only)"},
	{Name: "--source", Value: "ID", Repeatable: true, Summary: "Entry this one was derived from"},
	{Name: "--verified", Value: "DATE", Default: "today", Summary: "When the facts were checked (references only)"},
	{Name: "--review-days", Value: "N", Summary: "Staleness threshold for verified (references only)"},
	{Name: "--alias", Value: "A", Repeatable: true, Summary: "Former name or path that resolves to this entry"},
}

var (
	optTitle   = Option{Name: "--title", Value: "T", Required: true, Summary: "Title; write --title=-x for a value starting with -"}
	optSummary = Option{Name: "--summary", Value: "S", Required: true, Summary: "One sentence that tells a reader whether to open the entry"}
	optBody    = Option{Name: "--body", Value: "FILE", Required: true, Summary: "Body file, or - for standard input"}
	optJSON    = Option{Name: "--json", Summary: "Print one JSON object instead of text"}
	optAll     = Option{Name: "--all", Summary: "Make every status visible"}
)

var searchFilters = []Option{
	{Name: "--type", Value: "T", Repeatable: true, Summary: "Only this type (name or directory)"},
	{Name: "--tag", Value: "X", Repeatable: true, Summary: "Only entries with this tag"},
	{Name: "--repo", Value: "R", Repeatable: true, Summary: "Only entries concerning this repository"},
	{Name: "--host", Value: "H", Repeatable: true, Summary: "Only entries concerning this machine"},
	optAll,
	{Name: "--limit", Value: "N", Default: "search_limit (12)", Summary: "Maximum number of results"},
}

func join(groups ...[]Option) []Option {
	var out []Option
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func optional(o Option) Option {
	o.Required = false
	return o
}

// withDefault returns a copy of opts with the default of the option named name replaced.
func withDefault(opts []Option, name, def string) []Option {
	out := slices.Clone(opts)
	for i := range out {
		if out[i].Name == name {
			out[i].Default = def
		}
	}
	return out
}

var writeExits = []int{ExitOK, ExitFailure, ExitUsage, ExitLock, ExitNotPushed, ExitConflict}

// Commands is every command, in the order of SPEC §11 (§11.1 init to §11.17 update).
var Commands = []Command{
	{
		Name:    "init",
		Summary: "Create a store, or write pointer files into a directory",
		Usage:   "memex init PATH [--name NAME]\nmemex init --pointers DIR [--force]",
		Options: []Option{
			{Name: "--name", Value: "NAME", Default: "the directory basename", Summary: "Store name"},
			{Name: "--pointers", Value: "DIR", Summary: "Write CLAUDE.md, AGENTS.md and GEMINI.md pointer files into DIR"},
			{Name: "--force", Summary: "Replace differing pointer files, printing their old content"},
		},
		Positionals: []Positional{{Name: "PATH"}},
		ExitCodes:   []int{ExitOK, ExitFailure, ExitUsage},
	},
	{
		Name:    "new",
		Summary: "Create an entry of any type and commit it",
		Usage: "memex new TYPE --title=T --summary=S --body FILE|- [--status S] [--tag X]… [--keyword K]…\n" +
			"          [--repo R]… [--no-repo] [--host H]… [--always] [--kind K]\n" +
			"          [--parent ID] [--source ID]… [--verified DATE] [--review-days N] [--alias A]…",
		Mutating:    true,
		Options:     join([]Option{optTitle, optSummary, optBody}, newOptions),
		Positionals: []Positional{{Name: "TYPE", Required: true}},
		ExitCodes:   writeExits,
	},
	{
		Name:    "capture",
		Summary: "Create a journal entry and commit it",
		Usage: "memex capture --title=T --summary=S --body FILE|- [same options]\n" +
			"memex capture --from PATH [--title=T] [--summary=S] [same options]",
		Mutating: true,
		Options: join(
			[]Option{optional(optTitle), optional(optSummary), optional(optBody),
				{Name: "--from", Value: "PATH", Summary: "Import a file found by the leak scan"}},
			withDefault(withDefault(newOptions, "--status", "raw"), "--verified", "")),
		ExitCodes: writeExits,
	},
	{
		Name:        "search",
		Summary:     "Ranked full-text search of the store",
		Usage:       "memex search QUERY… [--type T]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N] [--json]",
		Options:     join(searchFilters, []Option{optJSON}),
		Positionals: []Positional{{Name: "QUERY", Repeatable: true, Required: true}},
		ExitCodes:   []int{ExitOK, ExitUsage, ExitNoMatch},
	},
	{
		Name:    "preflight",
		Summary: "Start-of-task ritual: sync, Always rules, then search",
		Usage:   "memex preflight [QUERY…] [--no-sync] [--max-chars N] [search options]",
		Options: join([]Option{
			{Name: "--no-sync", Summary: "Do not sync the store first"},
			{Name: "--max-chars", Value: "N", Default: "12000", Summary: "Bound on the whole output, in code points (minimum 2000)"},
		}, searchFilters),
		Positionals: []Positional{{Name: "QUERY", Repeatable: true}},
		ExitCodes:   []int{ExitOK, ExitUsage, ExitNoMatch},
	},
	{
		Name:    "list",
		Summary: "List entries, newest first",
		Usage: "memex list [--type T]… [--status S]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N]\n" +
			"           [--fields F,…] [--json]",
		Options: []Option{
			{Name: "--type", Value: "T", Repeatable: true, Summary: "Only this type (name or directory)"},
			{Name: "--status", Value: "S", Repeatable: true, Summary: "Only this status"},
			{Name: "--tag", Value: "X", Repeatable: true, Summary: "Only entries with this tag"},
			{Name: "--repo", Value: "R", Repeatable: true, Summary: "Only entries concerning this repository"},
			{Name: "--host", Value: "H", Repeatable: true, Summary: "Only entries concerning this machine"},
			{Name: "--all", Summary: "List every entry, whatever its status"},
			{Name: "--limit", Value: "N", Default: "list_limit (50)", Summary: "Maximum number of entries; 0 means no limit"},
			{Name: "--fields", Value: "F,…", Summary: "Columns to print, in order"},
			optJSON,
		},
		ExitCodes: []int{ExitOK, ExitUsage},
	},
	{
		Name:        "show",
		Summary:     "Print entries with their links",
		Usage:       "memex show ID… [--json]",
		Options:     []Option{optJSON},
		Positionals: []Positional{{Name: "ID", Repeatable: true, Required: true}},
		ExitCodes:   []int{ExitOK, ExitFailure, ExitUsage},
	},
	{
		Name:     "set",
		Summary:  "Change frontmatter fields of an entry and commit it",
		Usage:    "memex set ID ASSIGNMENT…\nmemex set --batch FILE|-",
		Mutating: true,
		Options: []Option{
			{Name: "--batch", Value: "FILE", Summary: "Apply one ID ASSIGNMENT… line of FILE (- for standard input) per entry, in one commit"},
		},
		Positionals: []Positional{
			{Name: "ID", Required: true},
			{Name: "ASSIGNMENT", Repeatable: true, Required: true},
		},
		ExitCodes: writeExits,
	},
	{
		Name:     "promote",
		Summary:  "Create an entry derived from another and mark a journal source promoted",
		Usage:    "memex promote SOURCE-ID --to TYPE --title=T --summary=S --body FILE|- [new options]",
		Mutating: true,
		Options: join([]Option{
			{Name: "--to", Value: "TYPE", Required: true, Summary: "Type of the new entry"},
			optTitle, optSummary, optBody,
		}, newOptions),
		Positionals: []Positional{{Name: "SOURCE-ID", Required: true}},
		ExitCodes:   writeExits,
	},
	{
		Name:     "fmt",
		Summary:  "Rewrite entries into canonical form and commit them",
		Usage:    "memex fmt [PATHS…] [--check]",
		Mutating: true,
		Options: []Option{
			{Name: "--check", Summary: "Write nothing; list non-canonical paths and exit 1 if any"},
		},
		Positionals: []Positional{{Name: "PATHS", Repeatable: true}},
		ExitCodes:   writeExits,
	},
	{
		Name:     "save",
		Summary:  "Commit direct edits to the store",
		Usage:    "memex save [-m MSG] [PATHS…] [--allow-delete] [--fmt]",
		Mutating: true,
		Options: []Option{
			{Name: "-m", Value: "MSG", Default: "memex: save <n> files", Summary: "Commit message"},
			{Name: "--allow-delete", Summary: "Allow committing deleted entries"},
			{Name: "--fmt", Summary: "Rewrite the selected entries into canonical form first"},
		},
		Positionals: []Positional{{Name: "PATHS", Repeatable: true}},
		ExitCodes:   writeExits,
	},
	{
		Name:      "sync",
		Summary:   "Fetch, fast-forward or rebase, and push the store",
		Usage:     "memex sync",
		Mutating:  true,
		ExitCodes: []int{ExitOK, ExitUsage, ExitLock, ExitNotPushed},
	},
	{
		Name:    "check",
		Summary: "Validate the store",
		Usage:   "memex check [--staged] [--workspace] [--strict] [--errors-only] [--limit N] [--immutable] [--json]",
		Options: []Option{
			{Name: "--staged", Summary: "Validate only entries added or modified in the git index"},
			{Name: "--workspace", Summary: "Also check pointer files and scan for leaks in tool memory (§12)"},
			{Name: "--strict", Summary: "Count warnings as errors"},
			{Name: "--errors-only", Summary: "Suppress ordinary warning lines"},
			{Name: "--limit", Value: "N", Default: "200", Summary: "Maximum number of finding lines; 0 means no limit"},
			{Name: "--immutable", Summary: "Also list the findings on immutable journal bodies; they never count as warnings"},
			optJSON,
		},
		ExitCodes: []int{ExitOK, ExitFailure, ExitUsage},
	},
	{
		Name:      "doctor",
		Summary:   "Check the installation, the store and tool neutrality",
		Usage:     "memex doctor [--json]",
		Options:   []Option{optJSON},
		ExitCodes: []int{ExitOK, ExitFailure, ExitUsage},
	},
	{
		Name:    "import",
		Summary: "Convert brain v3 stores into a new store, or add what is new to an existing one",
		Usage: "memex import brain-v3 SRC[@REV]… --out DIR [--leak PATH]…\n" +
			"memex import brain-v3 SRC[@REV]… --into DIR [--leak PATH]…",
		Options: []Option{
			{Name: "--out", Value: "DIR", Summary: "New store directory; must not exist"},
			{Name: "--into", Value: "DIR", Summary: "Existing store to add only the entries it does not hold yet"},
			{Name: "--leak", Value: "PATH", Repeatable: true, Summary: "Tool-memory file to import as a raw journal entry"},
		},
		Positionals: []Positional{
			{Name: "brain-v3", Required: true},
			{Name: "SRC[@REV]", Repeatable: true, Required: true},
		},
		ExitCodes: []int{ExitOK, ExitFailure, ExitUsage, ExitLock},
	},
	{
		Name:    "help",
		Summary: "Print help",
		Usage:   "memex help [COMMAND|TOPIC] [--all] [--json]",
		Options: []Option{
			{Name: "--all", Summary: "Print the overview, every topic and every command page"},
			{Name: "--json", Summary: "Print the machine-readable interface"},
		},
		Positionals: []Positional{{Name: "COMMAND|TOPIC"}},
		ExitCodes:   []int{ExitOK, ExitUsage},
	},
	{
		Name:      "version",
		Summary:   "Print the version",
		Usage:     "memex version",
		ExitCodes: []int{ExitOK, ExitUsage},
	},
	{
		Name:     "update",
		Summary:  "Append a dated section to an entry and replace its summary, in one commit",
		Usage:    "memex update ID --summary=S --body FILE|- [--heading=H] [--status S] [--no-repo]",
		Mutating: true,
		Options: []Option{
			{Name: "--summary", Value: "S", Required: true, Summary: "The summary as it stands after this section; must differ from the current one"},
			optBody,
			{Name: "--heading", Value: "H", Default: "Status", Summary: "Heading of the new section, 1 to 100 characters; the date is appended"},
			{Name: "--status", Value: "S", Summary: "New status, set in the same commit"},
			{Name: "--no-repo", Summary: "In a store with require_repos, the entry concerns no repository: write repos = []"},
		},
		Positionals: []Positional{{Name: "ID", Required: true}},
		ExitCodes:   writeExits,
	},
}

// LookupCommand returns the command named name.
func LookupCommand(name string) (*Command, bool) {
	for i := range Commands {
		if Commands[i].Name == name {
			return &Commands[i], true
		}
	}
	return nil, false
}

// CommandNames returns every command name, in table order.
func CommandNames() []string {
	names := make([]string, len(Commands))
	for i, c := range Commands {
		names[i] = c.Name
	}
	return names
}

// Topic is one help topic (SPEC §16.6).
type Topic struct {
	Name    string
	Summary string
}

// Topics is the SPEC §16.6 table, in its order, which is the `help --all` order and the
// tie-break order of help suggestions (§16.2).
var Topics = []Topic{
	{"agents", "Rules of engagement for language-model agents"},
	{"concepts", "The model and the reasoning behind it"},
	{"entries", "Types, statuses, frontmatter fields and body outlines"},
	{"ids", "The id and slug rules, with worked examples"},
	{"links", "Link syntax and resolution"},
	{"search", "Query syntax, ranking, filters and how to read output"},
	{"writing", "The write protocol, direct edits, outcomes and conflicts"},
	{"config", "Store resolution, the config file and environment variables"},
	{"check-codes", "Every E- and W- code, with its condition and the usual fix"},
	{"exit-codes", "The exit-code table"},
	{"neutrality", "Pointer files, the leak scan and tool memory"},
	{"importing", "The brain v3 importer and its report"},
}

// TopicNames returns every topic name, in table order.
func TopicNames() []string {
	names := make([]string, len(Topics))
	for i, t := range Topics {
		names[i] = t.Name
	}
	return names
}

// LookupField returns the §5.3 field named name, or nil.
func LookupField(name string) *Field {
	for i := range Fields {
		if Fields[i].Name == name {
			return &Fields[i]
		}
	}
	return nil
}

// FieldNames returns every field name, in canonical order.
func FieldNames() []string {
	names := make([]string, len(Fields))
	for i, f := range Fields {
		names[i] = f.Name
	}
	return names
}

// AllowedOnType reports whether the field may appear on entries of the named type.
func (f *Field) AllowedOnType(typ string) bool {
	for _, t := range f.AllowedOn {
		if t == AllTypes || t == typ {
			return true
		}
	}
	return false
}

// LookupType returns the type whose name or directory is s, or nil.
func LookupType(s string) *Type {
	for i := range Types {
		if Types[i].Name == s || Types[i].Directory == s {
			return &Types[i]
		}
	}
	return nil
}

// LookupDirectory returns the type whose directory is dir, or nil.
func LookupDirectory(dir string) *Type {
	for i := range Types {
		if Types[i].Directory == dir {
			return &Types[i]
		}
	}
	return nil
}
