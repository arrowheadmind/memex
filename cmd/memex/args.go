package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/pflag"

	"github.com/arrowheadmind/memex/internal/spec"
)

// args are a command's parsed options and positionals.
type args struct {
	values      map[string][]string // option name → values in order; a flag has one ""
	positionals []string
}

func (a *args) has(name string) bool { return len(a.values[name]) > 0 }

func (a *args) value(name string) (string, bool) {
	v := a.values[name]
	if len(v) == 0 {
		return "", false
	}
	return v[len(v)-1], true
}

// noValue is the pflag value of an option that takes none. pflag passes NoOptDefVal when the
// option is written bare; anything else came from --opt=value, which the pre-scan refuses.
type noValue struct{ set bool }

const noValueMark = "\x00"

func (v *noValue) String() string { return "" }
func (v *noValue) Type() string   { return "bool" }
func (v *noValue) Set(s string) error {
	if s != noValueMark {
		return fmt.Errorf("takes no value")
	}
	v.set = true
	return nil
}

// commandOptions are the options a command accepts after its name: its own, and --store,
// which is accepted anywhere (SPEC §10).
func commandOptions(cmd *spec.Command) []spec.Option {
	opts := slices.Clone(cmd.Options)
	for _, g := range spec.GlobalOptions {
		if g.Name == "--store" {
			opts = append(opts, g)
		}
	}
	return opts
}

// parseArgs parses a command's arguments (SPEC §10): options anywhere before "--", long
// options only (plus -m where defined), --opt=value or --opt value, and never a value that
// starts with "-" unless written --opt=-x. argv is pre-scanned for those rules, so that every
// usage error has memex's wording; pflag then does the parsing.
func parseArgs(cmd *spec.Command, argv []string) (*args, string) {
	opts := commandOptions(cmd)
	lookup := func(name string) *spec.Option {
		for i := range opts {
			if opts[i].Name == name {
				return &opts[i]
			}
		}
		return nil
	}
	var names []string
	for _, o := range opts {
		names = append(names, o.Name)
	}
	count := map[string]int{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			continue
		}
		name, value, hasValue := strings.Cut(a, "=")
		o := lookup(name)
		if o == nil {
			return nil, unknownOption(name, names)
		}
		count[name]++
		if count[name] > 1 && !o.Repeatable {
			return nil, fmt.Sprintf("option %s given more than once", name)
		}
		switch {
		case o.Value == "" && hasValue:
			return nil, fmt.Sprintf("option %s takes no value", name)
		case o.Value != "" && !hasValue:
			if i+1 >= len(argv) {
				return nil, fmt.Sprintf("missing value for %s", name)
			}
			i++
			value = argv[i]
			if strings.HasPrefix(value, "-") && value != "-" {
				return nil, fmt.Sprintf("value for %s starts with \"-\"; write %s=%s", name, name, value)
			}
		}
		if name == "--store" && value == "" {
			return nil, "empty value for --store"
		}
	}

	fs := pflag.NewFlagSet(cmd.Name, pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.SetInterspersed(true)
	strs := map[string]*string{}
	arrays := map[string]*[]string{}
	flags := map[string]*noValue{}
	for _, o := range opts {
		long, short := strings.TrimPrefix(o.Name, "--"), ""
		if !strings.HasPrefix(o.Name, "--") {
			long, short = o.Name[1:], o.Name[1:]
		}
		switch {
		case o.Value == "":
			v := &noValue{}
			fs.VarPF(v, long, short, "").NoOptDefVal = noValueMark
			flags[o.Name] = v
		case o.Repeatable:
			arrays[o.Name] = fs.StringArrayP(long, short, nil, "")
		default:
			strs[o.Name] = fs.StringP(long, short, "", "")
		}
	}
	if err := fs.Parse(argv); err != nil {
		// The pre-scan admits only what pflag accepts; a difference is a bug.
		panic(fmt.Sprintf("pflag refused what the pre-scan admitted: %v", err))
	}
	res := &args{values: map[string][]string{}, positionals: fs.Args()}
	for _, o := range opts {
		switch {
		case flags[o.Name] != nil:
			if flags[o.Name].set {
				res.values[o.Name] = []string{""}
			}
		case arrays[o.Name] != nil:
			if len(*arrays[o.Name]) > 0 {
				res.values[o.Name] = *arrays[o.Name]
			}
		default:
			long := strings.TrimLeft(o.Name, "-")
			if fs.Changed(long) {
				res.values[o.Name] = []string{*strs[o.Name]}
			}
		}
	}
	if max := maxPositionals(cmd); max >= 0 && len(res.positionals) > max {
		return nil, fmt.Sprintf("unexpected argument %q", res.positionals[max])
	}
	return res, ""
}

// maxPositionals is the number of positionals cmd accepts, or -1 for no bound.
func maxPositionals(cmd *spec.Command) int {
	for _, p := range cmd.Positionals {
		if p.Repeatable {
			return -1
		}
	}
	return len(cmd.Positionals)
}
