package spec

import (
	"slices"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Commands {
		if seen[c.Name] {
			t.Errorf("duplicate command %q", c.Name)
		}
		seen[c.Name] = true
		if c.Summary == "" {
			t.Errorf("%s: no summary", c.Name)
		}
		for _, line := range strings.Split(c.Usage, "\n") {
			if !strings.HasPrefix(line, "memex "+c.Name) && !strings.HasPrefix(line, "  ") {
				t.Errorf("%s: usage line %q neither names the command nor continues", c.Name, line)
			}
		}
		opts := map[string]bool{}
		for _, o := range c.Options {
			if opts[o.Name] {
				t.Errorf("%s: duplicate option %s", c.Name, o.Name)
			}
			opts[o.Name] = true
			if o.Summary == "" {
				t.Errorf("%s %s: no summary", c.Name, o.Name)
			}
			if !strings.HasPrefix(o.Name, "--") && o.Name != "-m" {
				t.Errorf("%s %s: only -m and -h may be short (SPEC §10)", c.Name, o.Name)
			}
			if slices.Contains(HelpOptions, o.Name) {
				t.Errorf("%s %s: help options are implicit", c.Name, o.Name)
			}
		}
		if len(c.ExitCodes) == 0 || c.ExitCodes[0] != ExitOK || !slices.Contains(c.ExitCodes, ExitUsage) {
			t.Errorf("%s: exit codes %v must start with 0 and include 2", c.Name, c.ExitCodes)
		}
		for _, code := range c.ExitCodes {
			if !slices.ContainsFunc(ExitCodes, func(e ExitCode) bool { return e.Code == code }) {
				t.Errorf("%s: exit code %d is not in the §10 table", c.Name, code)
			}
		}
		if !slices.IsSorted(c.ExitCodes) {
			t.Errorf("%s: exit codes %v not sorted", c.Name, c.ExitCodes)
		}
	}
}

func TestTypesAndFields(t *testing.T) {
	names := map[string]bool{}
	for _, ty := range Types {
		names[ty.Name] = true
		for _, v := range ty.Visible {
			if !slices.Contains(ty.Statuses, v) {
				t.Errorf("%s: visible status %q is not a status", ty.Name, v)
			}
		}
	}
	for _, f := range Fields {
		for _, on := range f.AllowedOn {
			if on != AllTypes && !names[on] {
				t.Errorf("field %s: unknown type %q", f.Name, on)
			}
		}
		switch f.Kind {
		case KindString, KindDate, KindInteger, KindBoolean, KindArray:
		default:
			t.Errorf("field %s: unknown kind %q", f.Name, f.Kind)
		}
	}
}

func TestLookupCommand(t *testing.T) {
	if c, ok := LookupCommand("version"); !ok || c.Name != "version" {
		t.Error("version not found")
	}
	if _, ok := LookupCommand("nope"); ok {
		t.Error("nope found")
	}
}
