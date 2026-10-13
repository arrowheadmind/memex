package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/arrowheadmind/memex/internal/commands"
	"github.com/arrowheadmind/memex/internal/search"
	"github.com/arrowheadmind/memex/internal/spec"
)

// typeNames are the names and directories --type accepts, for suggestions.
func typeNames() []string {
	var names []string
	for _, t := range spec.Types {
		names = append(names, t.Name)
	}
	for _, t := range spec.Types {
		if t.Directory != t.Name {
			names = append(names, t.Directory)
		}
	}
	return names
}

// unknownValue describes a value that is not one of candidates (SPEC §17.4).
func unknownValue(what, v string, candidates []string) string {
	problem := fmt.Sprintf("unknown %s %q", what, v)
	if near, ok := spec.Nearest(v, candidates); ok {
		return problem + fmt.Sprintf("; did you mean %q?", near)
	}
	return problem + "; valid: " + strings.Join(candidates, ", ")
}

// parseFilter reads --type, --tag, --repo, --host and --all (SPEC §8.4).
func parseFilter(a *args) (search.Filter, string) {
	f := search.Filter{
		Tags:  a.values["--tag"],
		Repos: a.values["--repo"],
		Hosts: a.values["--host"],
		All:   a.has("--all"),
	}
	for _, v := range a.values["--type"] {
		t := spec.LookupType(v)
		if t == nil {
			return f, unknownValue("type", v, typeNames())
		}
		f.Types = append(f.Types, t)
	}
	return f, ""
}

// parseLimit reads --limit: a non-negative integer, 0 meaning no limit; -1 when absent.
func parseLimit(a *args) (int, string) {
	v, ok := a.value("--limit")
	if !ok {
		return -1, ""
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || strings.HasPrefix(v, "+") {
		return 0, fmt.Sprintf("value for --limit must be a non-negative integer, not %q", v)
	}
	return n, ""
}

// parseQuery joins the positionals into the query (SPEC §11.3) and parses it (§8.3).
func parseQuery(positionals []string) (search.Query, string) {
	q := search.ParseQuery(strings.Join(positionals, " "))
	if len(q.Items) == 0 {
		return q, "the query has no terms or phrases; give at least one word"
	}
	return q, ""
}

func runSearch(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if len(a.positionals) == 0 {
		return 0, cmdErr("missing QUERY")
	}
	q, problem := parseQuery(a.positionals)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	f, problem := parseFilter(a)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	limit, problem := parseLimit(a)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	return commands.Search(env, commands.SearchOptions{Query: q, Filter: f, Limit: limit, JSON: a.has("--json")}), nil
}

func runList(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	f, problem := parseFilter(a)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	limit, problem := parseLimit(a)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	o := commands.ListOptions{Filter: f, Limit: limit, JSON: a.has("--json")}
	var statuses []string
	for _, t := range spec.Types {
		for _, s := range t.Statuses {
			if !slices.Contains(statuses, s) {
				statuses = append(statuses, s)
			}
		}
	}
	for _, s := range a.values["--status"] {
		if !slices.Contains(statuses, s) {
			return 0, cmdErr(unknownValue("status", s, statuses))
		}
		o.Statuses = append(o.Statuses, s)
	}
	if v, ok := a.value("--fields"); ok {
		o.Fields = []string{}
		for _, name := range strings.Split(v, ",") {
			if !slices.Contains(commands.ListFields, name) {
				return 0, cmdErr(unknownValue("field", name, commands.ListFields))
			}
			o.Fields = append(o.Fields, name)
		}
	}
	return commands.List(env, o), nil
}

func runShow(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if len(a.positionals) == 0 {
		return 0, cmdErr("missing ID")
	}
	return commands.Show(env, commands.ShowOptions{IDs: a.positionals, JSON: a.has("--json")}), nil
}

func runPreflight(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	o := commands.PreflightOptions{NoSync: a.has("--no-sync"), MaxChars: commands.DefaultMaxChars}
	if len(a.positionals) > 0 {
		q, problem := parseQuery(a.positionals)
		if problem != "" {
			return 0, cmdErr(problem)
		}
		o.Query = &q
	}
	var problem string
	if o.Filter, problem = parseFilter(a); problem != "" {
		return 0, cmdErr(problem)
	}
	if o.Limit, problem = parseLimit(a); problem != "" {
		return 0, cmdErr(problem)
	}
	if v, ok := a.value("--max-chars"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < commands.MinMaxChars || strings.HasPrefix(v, "+") {
			return 0, cmdErr(fmt.Sprintf("value for --max-chars must be an integer of at least %d, not %q", commands.MinMaxChars, v))
		}
		o.MaxChars = n
	}
	code, uerr := commands.Preflight(env, o)
	if uerr != nil {
		return 0, cmdErr(uerr.Problem)
	}
	return code, nil
}
