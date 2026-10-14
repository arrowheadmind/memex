package main

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/arrowheadmind/memex/help"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/spec"
)

// printTopics prints one line per command and topic that has a page: the name, two spaces and
// its one-line description (SPEC §16.2).
func printTopics(stdout io.Writer) (int, *usageError) {
	for _, c := range spec.Commands {
		if hasPage("cmd-" + c.Name) {
			fmt.Fprintf(stdout, "%s  %s\n", c.Name, c.Summary)
		}
	}
	for _, t := range spec.Topics {
		if hasPage("topic-" + t.Name) {
			fmt.Fprintf(stdout, "%s  %s\n", t.Name, t.Summary)
		}
	}
	return spec.ExitOK, nil
}

// printAll prints the overview, every topic page, then every command page, in the order of
// SPEC §16.6 and §11, separated by a line `---` (SPEC §16.2).
func printAll(stdout io.Writer) (int, *usageError) {
	printOverview(stdout)
	for _, t := range spec.Topics {
		printPageAfterSeparator(stdout, "topic-"+t.Name)
	}
	for _, c := range spec.Commands {
		printPageAfterSeparator(stdout, "cmd-"+c.Name)
	}
	return spec.ExitOK, nil
}

func printPageAfterSeparator(stdout io.Writer, file string) {
	b, err := fs.ReadFile(help.Pages, file+".md")
	if err != nil {
		return
	}
	io.WriteString(stdout, "---\n")
	stdout.Write(b)
}

func hasPage(file string) bool {
	_, err := fs.Stat(help.Pages, file+".md")
	return err == nil
}

// printInterface prints the machine-readable interface (SPEC §16.7), generated from the
// internal/spec tables that also drive the parser and the validator.
func printInterface(stdout io.Writer) (int, *usageError) {
	cmds := make(jsonx.Array, 0, len(spec.Commands))
	for _, c := range spec.Commands {
		opts := make(jsonx.Array, 0, len(c.Options))
		for _, o := range c.Options {
			opts = append(opts, optionJSON(o, requiredValue(c, o)))
		}
		pos := make(jsonx.Array, 0, len(c.Positionals))
		for _, p := range c.Positionals {
			pos = append(pos, jsonx.Object{
				{Key: "name", Value: jsonx.String(p.Name)},
				{Key: "repeatable", Value: jsonx.Bool(p.Repeatable)},
				{Key: "required", Value: jsonx.Bool(p.Required)},
			})
		}
		exits := make(jsonx.Array, 0, len(c.ExitCodes))
		for _, e := range c.ExitCodes {
			exits = append(exits, jsonx.Int(e))
		}
		cmds = append(cmds, jsonx.Object{
			{Key: "name", Value: jsonx.String(c.Name)},
			{Key: "summary", Value: jsonx.String(c.Summary)},
			{Key: "usage", Value: jsonx.String(c.Usage)},
			{Key: "mutating", Value: jsonx.Bool(c.Mutating)},
			{Key: "options", Value: opts},
			{Key: "positionals", Value: pos},
			{Key: "exit_codes", Value: exits},
		})
	}
	types := make(jsonx.Array, 0, len(spec.Types))
	for _, t := range spec.Types {
		types = append(types, jsonx.Object{
			{Key: "name", Value: jsonx.String(t.Name)},
			{Key: "directory", Value: jsonx.String(t.Directory)},
			{Key: "statuses", Value: jsonx.Strings(t.Statuses)},
			{Key: "visible", Value: jsonx.Strings(t.Visible)},
		})
	}
	fields := make(jsonx.Array, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		fields = append(fields, jsonx.Object{
			{Key: "name", Value: jsonx.String(f.Name)},
			{Key: "kind", Value: jsonx.String(f.Kind)},
			{Key: "required", Value: jsonx.String(f.Required)},
			{Key: "allowed_on", Value: jsonx.Strings(f.AllowedOn)},
		})
	}
	exitCodes := make(jsonx.Array, 0, len(spec.ExitCodes))
	for _, e := range spec.ExitCodes {
		exitCodes = append(exitCodes, jsonx.Object{
			{Key: "code", Value: jsonx.Int(e.Code)},
			{Key: "meaning", Value: jsonx.String(e.Meaning)},
		})
	}
	topics := make(jsonx.Array, 0, len(spec.Topics))
	for _, t := range spec.Topics {
		topics = append(topics, jsonx.Object{
			{Key: "name", Value: jsonx.String(t.Name)},
			{Key: "summary", Value: jsonx.String(t.Summary)},
		})
	}
	// Global options accepted before and after every command: --store (SPEC §10, §16.7).
	globals := jsonx.Array{}
	for _, o := range spec.GlobalOptions {
		if o.Name == "--store" {
			globals = append(globals, optionJSON(o, jsonx.Bool(o.Required)))
		}
	}
	stdout.Write(jsonx.Encode(jsonx.Object{
		{Key: "version", Value: jsonx.String(version)},
		{Key: "format", Value: jsonx.Int(spec.StoreFormat)},
		{Key: "global_options", Value: globals},
		{Key: "commands", Value: cmds},
		{Key: "types", Value: types},
		{Key: "fields", Value: fields},
		{Key: "exit_codes", Value: exitCodes},
		{Key: "topics", Value: topics},
	}))
	return spec.ExitOK, nil
}

func stringOrNull(s string) jsonx.Value {
	if s == "" {
		return jsonx.Null
	}
	return jsonx.String(s)
}

// optionJSON is one option of the interface (SPEC §16.7).
func optionJSON(o spec.Option, required jsonx.Value) jsonx.Object {
	return jsonx.Object{
		{Key: "name", Value: jsonx.String(o.Name)},
		{Key: "value", Value: stringOrNull(o.Value)},
		{Key: "repeatable", Value: jsonx.Bool(o.Repeatable)},
		{Key: "required", Value: required},
		{Key: "default", Value: stringOrNull(o.Default)},
		{Key: "summary", Value: jsonx.String(o.Summary)},
	}
}

// requiredValue is true, false, or a condition string (SPEC §16.7): capture's --title,
// --summary and --body are required unless --from is given.
func requiredValue(c spec.Command, o spec.Option) jsonx.Value {
	if c.Name == "capture" && (o.Name == "--title" || o.Name == "--summary" || o.Name == "--body") {
		return jsonx.String("unless --from")
	}
	return jsonx.Bool(o.Required)
}

// runHelp implements `memex help [COMMAND|TOPIC] [--all] [--json]` (SPEC §16.2).
func runHelp(args []string, stdout io.Writer, cmdErr func(string) *usageError) (int, *usageError) {
	cmd, _ := spec.LookupCommand("help")
	var options []string
	for _, o := range cmd.Options {
		options = append(options, o.Name)
	}
	var names []string
	all, asJSON := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			name, _, hasValue := strings.Cut(a, "=")
			switch name {
			case "--all", "--json":
				if hasValue {
					return 0, cmdErr(fmt.Sprintf("option %s takes no value", name))
				}
				if (name == "--all" && all) || (name == "--json" && asJSON) {
					return 0, cmdErr(fmt.Sprintf("option %s given more than once", name))
				}
				if name == "--all" {
					all = true
				} else {
					asJSON = true
				}
				continue
			}
			return 0, cmdErr(unexpected(a, options))
		}
		names = append(names, a)
	}
	if all && asJSON {
		return 0, cmdErr("--all and --json cannot be combined")
	}
	if (all || asJSON) && len(names) > 0 {
		return 0, cmdErr(fmt.Sprintf("unexpected argument %q", names[0]))
	}
	switch {
	case all:
		return printAll(stdout)
	case asJSON:
		return printInterface(stdout)
	}
	switch len(names) {
	case 0:
		return printOverview(stdout)
	case 1:
		if names[0] == "topics" {
			return printTopics(stdout)
		}
		return printHelpPage(names[0], stdout)
	default:
		return 0, cmdErr(unexpected(names[1], nil))
	}
}
