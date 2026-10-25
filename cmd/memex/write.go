package main

import (
	"fmt"

	"github.com/arrowheadmind/memex/internal/commands"
	"github.com/arrowheadmind/memex/internal/spec"
)

// newOptions reads the options shared by new, capture and promote (SPEC §11.2).
func newOptions(a *args) commands.NewOptions {
	o := commands.NewOptions{
		Tags: a.values["--tag"], Keywords: a.values["--keyword"], Repos: a.values["--repo"],
		Hosts: a.values["--host"], Sources: a.values["--source"], Alias: a.values["--alias"],
		NoRepo: a.has("--no-repo"), Always: a.has("--always"),
	}
	o.Title, o.TitleSet = a.value("--title")
	o.Summary, o.SummarySet = a.value("--summary")
	o.Body, _ = a.value("--body")
	o.From, _ = a.value("--from")
	o.Status, _ = a.value("--status")
	o.Kind, _ = a.value("--kind")
	o.Parent, _ = a.value("--parent")
	o.Verified, _ = a.value("--verified")
	o.ReviewDays, _ = a.value("--review-days")
	return o
}

// requireNew reports the first missing option of title, summary and body.
func requireNew(a *args) string {
	for _, name := range []string{"--title", "--summary", "--body"} {
		if !a.has(name) {
			return "missing " + name
		}
	}
	return ""
}

func lookupType(v string) (*spec.Type, string) {
	t := spec.LookupType(v)
	if t == nil {
		return nil, unknownValue("type", v, typeNames())
	}
	return t, ""
}

// usage turns a command's result into a runner's: a *commands.UsageError becomes the
// §16.8 usage error.
func usage(code int, uerr *commands.UsageError) func(func(string) *usageError) (int, *usageError) {
	return func(cmdErr func(string) *usageError) (int, *usageError) {
		if uerr != nil {
			return 0, cmdErr(uerr.Problem)
		}
		return code, nil
	}
}

func runNew(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if len(a.positionals) == 0 {
		return 0, cmdErr("missing TYPE")
	}
	t, problem := lookupType(a.positionals[0])
	if problem != "" {
		return 0, cmdErr(problem)
	}
	if problem := requireNew(a); problem != "" {
		return 0, cmdErr(problem)
	}
	o := newOptions(a)
	o.Type = t
	return usage(commands.New(env, o))(cmdErr)
}

func runCapture(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if a.has("--from") {
		if a.has("--body") {
			return 0, cmdErr("--from and --body cannot be combined; --from PATH supplies the body")
		}
	} else if problem := requireNew(a); problem != "" {
		return 0, cmdErr(problem)
	}
	o := newOptions(a)
	o.Type = spec.LookupType("journal")
	return usage(commands.New(env, o))(cmdErr)
}

func runPromote(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if len(a.positionals) == 0 {
		return 0, cmdErr("missing SOURCE-ID")
	}
	to, ok := a.value("--to")
	if !ok {
		return 0, cmdErr("missing --to")
	}
	t, problem := lookupType(to)
	if problem != "" {
		return 0, cmdErr(problem)
	}
	if problem := requireNew(a); problem != "" {
		return 0, cmdErr(problem)
	}
	o := commands.PromoteOptions{Source: a.positionals[0], New: newOptions(a)}
	o.New.Type = t
	return usage(commands.Promote(env, o))(cmdErr)
}

func runSet(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if file, ok := a.value("--batch"); ok {
		if len(a.positionals) > 0 {
			return 0, cmdErr(fmt.Sprintf("unexpected argument %q; with --batch, each line of FILE holds ID ASSIGNMENT…", a.positionals[0]))
		}
		return usage(commands.SetBatch(env, file))(cmdErr)
	}
	switch len(a.positionals) {
	case 0:
		return 0, cmdErr("missing ID")
	case 1:
		return 0, cmdErr("missing ASSIGNMENT; write field=value, field=, field+=value or field-=value")
	}
	o := commands.SetOptions{ID: a.positionals[0]}
	for _, arg := range a.positionals[1:] {
		asg, problem := commands.ParseAssignment(arg)
		if problem != "" {
			return 0, cmdErr(problem)
		}
		o.Assignments = append(o.Assignments, asg)
	}
	return usage(commands.Set(env, o))(cmdErr)
}

func runUpdate(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	switch len(a.positionals) {
	case 0:
		return 0, cmdErr("missing ID")
	case 1:
	default:
		return 0, cmdErr(unexpected(a.positionals[1], nil))
	}
	for _, name := range []string{"--summary", "--body"} {
		if !a.has(name) {
			return 0, cmdErr("missing " + name)
		}
	}
	o := commands.UpdateOptions{ID: a.positionals[0]}
	o.Summary, _ = a.value("--summary")
	o.Body, _ = a.value("--body")
	o.Status, _ = a.value("--status")
	if o.Summary == "" {
		return 0, cmdErr("empty value for --summary; write the summary as it stands after this section")
	}
	if h, ok := a.value("--heading"); ok {
		if problem := commands.CheckHeading(h); problem != "" {
			return 0, cmdErr(problem)
		}
		o.Heading = h
	}
	return usage(commands.Update(env, o))(cmdErr)
}

func runFmt(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	return commands.Fmt(env, commands.FmtOptions{Paths: a.positionals, Check: a.has("--check")}), nil
}

func runSave(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	o := commands.SaveOptions{Paths: a.positionals, AllowDelete: a.has("--allow-delete"), Fmt: a.has("--fmt")}
	o.Message, o.MessageSet = a.value("-m")
	return usage(commands.Save(env, o))(cmdErr)
}

func runSync(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	if len(a.positionals) > 0 {
		return 0, cmdErr(unexpected(a.positionals[0], nil))
	}
	return commands.Sync(env), nil
}

func runInit(env *commands.Env, a *args, cmdErr func(string) *usageError) (int, *usageError) {
	o := commands.InitOptions{Force: a.has("--force")}
	o.Pointers, _ = a.value("--pointers")
	o.Name, o.NameSet = a.value("--name")
	if a.has("--pointers") {
		switch {
		case o.Pointers == "":
			return 0, cmdErr("empty value for --pointers")
		case len(a.positionals) > 0:
			return 0, cmdErr("give either PATH or --pointers DIR, not both")
		case o.NameSet:
			return 0, cmdErr("--name belongs to the store form, memex init PATH")
		}
		return commands.Init(env, o), nil
	}
	if len(a.positionals) == 0 {
		return 0, cmdErr("missing PATH")
	}
	if o.Force {
		return 0, cmdErr("--force belongs to the pointer form, memex init --pointers DIR")
	}
	o.Path = a.positionals[0]
	return commands.Init(env, o), nil
}
