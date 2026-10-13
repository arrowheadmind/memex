// Command memex is the memex command-line tool. SPEC.md is its specification.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/arrowheadmind/memex/help"
	"github.com/arrowheadmind/memex/internal/commands"
	"github.com/arrowheadmind/memex/internal/spec"
)

// version is set at build time with -ldflags "-X main.version=…" (IMPLEMENTATION.md).
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// usageError is a SPEC §16.8 usage error: the problem, the usage line and the help command.
// Without a usage line it is the one-line form of §16.2.
type usageError struct {
	problem string
	usage   string
	helpRef string
}

// run executes one invocation and returns its exit code. A panic is a bug: it is reported
// and exits 70 (SPEC §10).
func run(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "memex: internal error: %v\n", r)
			code = spec.ExitInternal
		}
	}()
	code, err := dispatchFunc(args, stdout, stderr)
	if err != nil {
		if err.usage == "" {
			// An unknown name has no usage line; the message is one line (SPEC §16.2).
			fmt.Fprintf(stderr, "memex: %s Run: %s\n", err.problem, err.helpRef)
		} else {
			fmt.Fprintf(stderr, "memex: %s\n%s\nRun: %s\n", err.problem, err.usage, err.helpRef)
		}
		return spec.ExitUsage
	}
	return code
}

// dispatchFunc is dispatch, replaceable by tests to exercise panic recovery.
var dispatchFunc = dispatch

func dispatch(args []string, stdout, stderr io.Writer) (int, *usageError) {
	env := commands.OSEnv(stdout, stderr)
	globalErr := func(problem string) *usageError {
		return &usageError{problem, spec.GlobalUsage, "memex help"}
	}
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		name, value, hasValue := strings.Cut(a, "=")
		switch name {
		case "--help", "-h":
			if hasValue {
				return 0, globalErr(fmt.Sprintf("option %s takes no value", name))
			}
			return printOverview(stdout)
		case "--version":
			if hasValue {
				return 0, globalErr("option --version takes no value")
			}
			return printVersion(stdout)
		case "--store":
			if env.StoreSet {
				return 0, globalErr("option --store given more than once")
			}
			if !hasValue {
				if i+1 >= len(args) {
					return 0, globalErr("missing value for --store")
				}
				i++
				value = args[i]
				if strings.HasPrefix(value, "-") {
					return 0, globalErr(fmt.Sprintf("value for --store starts with \"-\"; write --store=%s", value))
				}
			}
			if value == "" {
				return 0, globalErr("empty value for --store")
			}
			// The store is resolved by the commands that need one (SPEC §3.1).
			env.Store, env.StoreSet = value, true
		default:
			return 0, globalErr(unknownOption(name, globalOptionNames()))
		}
	}
	if i >= len(args) {
		return 0, globalErr("missing command")
	}
	name, rest := args[i], args[i+1:]
	cmd, ok := spec.LookupCommand(name)
	if !ok {
		return 0, &usageError{problem: unknownCommand(name), helpRef: "memex help topics"}
	}
	cmdErr := func(problem string) *usageError {
		return &usageError{problem, cmd.Usage, "memex help " + cmd.Name}
	}
	if wantsHelp(rest) {
		return printHelpPage(cmd.Name, stdout)
	}
	switch cmd.Name {
	case "version":
		if len(rest) > 0 {
			return 0, cmdErr(unexpected(rest[0], nil))
		}
		return printVersion(stdout)
	case "help":
		return runHelp(rest, stdout, cmdErr)
	case "check":
		a, problem := parseCommand(cmd, rest, env)
		if problem != "" {
			return 0, cmdErr(problem)
		}
		return runCheck(env, a, cmdErr)
	default:
		return 0, cmdErr(fmt.Sprintf("command %q is not implemented yet", cmd.Name))
	}
}

// parseCommand parses a command's arguments and takes --store from them, which may appear
// after the command as well as before it, but only once.
func parseCommand(cmd *spec.Command, argv []string, env *commands.Env) (*args, string) {
	a, problem := parseArgs(cmd, argv)
	if problem != "" {
		return nil, problem
	}
	if v, ok := a.value("--store"); ok {
		if env.StoreSet {
			return nil, "option --store given more than once"
		}
		env.Store, env.StoreSet = v, true
	}
	return a, ""
}

func runCheck(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if a.has("--workspace") {
		return 0, cmdErr("option --workspace is not implemented yet")
	}
	o := commands.CheckOptions{
		Staged:     a.has("--staged"),
		Strict:     a.has("--strict"),
		ErrorsOnly: a.has("--errors-only"),
		JSON:       a.has("--json"),
		Limit:      commands.DefaultCheckLimit,
	}
	if v, ok := a.value("--limit"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || strings.HasPrefix(v, "+") {
			return 0, cmdErr(fmt.Sprintf("value for --limit must be a non-negative integer, not %q", v))
		}
		o.Limit = n
	}
	return commands.Check(env, o), nil
}

// wantsHelp reports whether --help or -h appears among the options (before any "--").
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if slices.Contains(spec.HelpOptions, a) {
			return true
		}
	}
	return false
}

func runHelp(args []string, stdout io.Writer, cmdErr func(string) *usageError) (int, *usageError) {
	cmd, _ := spec.LookupCommand("help")
	var options []string
	for _, o := range cmd.Options {
		options = append(options, o.Name)
	}
	var names []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			name, _, _ := strings.Cut(a, "=")
			if slices.Contains(options, name) {
				return 0, cmdErr(fmt.Sprintf("option %s is not implemented yet", name))
			}
			return 0, cmdErr(unexpected(a, options))
		}
		names = append(names, a)
	}
	switch len(names) {
	case 0:
		return printOverview(stdout)
	case 1:
		return printHelpPage(names[0], stdout)
	default:
		return 0, cmdErr(unexpected(names[1], nil))
	}
}

// printHelpPage prints the command or topic page for name (SPEC §16.2). No page exists yet.
func printHelpPage(name string, stdout io.Writer) (int, *usageError) {
	pages := pageNames()
	for _, file := range []string{"cmd-" + name + ".md", "topic-" + name + ".md"} {
		if b, err := fs.ReadFile(help.Pages, file); err == nil {
			stdout.Write(b)
			return spec.ExitOK, nil
		}
	}
	problem := fmt.Sprintf("no help for %q", name)
	if near, ok := spec.Nearest(name, pages); ok {
		problem += fmt.Sprintf("; did you mean %q?", near)
	} else {
		problem += "."
	}
	return 0, &usageError{problem: problem, helpRef: "memex help topics"}
}

// pageNames lists the commands and topics that have a help page, commands in §11 order, then
// topics in §16.6 order (SPEC §16.2).
func pageNames() []string {
	var names []string
	for _, c := range spec.Commands {
		if _, err := fs.Stat(help.Pages, "cmd-"+c.Name+".md"); err == nil {
			names = append(names, c.Name)
		}
	}
	for _, t := range spec.Topics {
		if _, err := fs.Stat(help.Pages, "topic-"+t.Name+".md"); err == nil {
			names = append(names, t.Name)
		}
	}
	return names
}

// printOverview prints help/overview.md, whose first line is replaced by the version line
// (SPEC §16.3, §16.4).
func printOverview(stdout io.Writer) (int, *usageError) {
	b, err := fs.ReadFile(help.Pages, "overview.md")
	if err != nil {
		panic(err)
	}
	_, body, _ := strings.Cut(string(b), "\n")
	fmt.Fprintf(stdout, "# memex %s (store format %d)\n%s", version, spec.StoreFormat, body)
	return spec.ExitOK, nil
}

func printVersion(stdout io.Writer) (int, *usageError) {
	fmt.Fprintf(stdout, "memex %s (store format %d)\n", version, spec.StoreFormat)
	return spec.ExitOK, nil
}

func unknownCommand(name string) string {
	problem := fmt.Sprintf("unknown command %q", name)
	if near, ok := spec.Nearest(name, spec.CommandNames()); ok {
		problem += fmt.Sprintf("; did you mean %q?", near)
	} else {
		problem += "."
	}
	return problem
}

func unknownOption(name string, candidates []string) string {
	problem := "unknown option " + name
	if near, ok := spec.Nearest(name, candidates); ok {
		problem += fmt.Sprintf(" (did you mean %s?)", near)
	}
	return problem
}

// unexpected describes an argument the command does not accept: an unknown option, or a
// positional where none is allowed.
func unexpected(arg string, options []string) string {
	if strings.HasPrefix(arg, "-") && arg != "-" {
		name, _, _ := strings.Cut(arg, "=")
		return unknownOption(name, options)
	}
	return fmt.Sprintf("unexpected argument %q", arg)
}

func globalOptionNames() []string {
	names := []string{}
	for _, o := range spec.GlobalOptions {
		names = append(names, o.Name)
	}
	return names
}
