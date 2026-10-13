package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/arrowheadmind/memex/internal/spec"
)

func invoke(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestRun(t *testing.T) {
	const ver = "memex 0.0.0-dev (store format 1)\n"
	const overviewHead = "# memex 0.0.0-dev (store format 1)\n\nmemex is "
	cases := []struct {
		name   string
		args   []string
		stdout string // exact, or prefix when prefix is set
		prefix bool
		stderr string
		code   int
	}{
		{"version", []string{"version"}, ver, false, "", 0},
		{"--version", []string{"--version"}, ver, false, "", 0},
		{"global store then version", []string{"--store", "/x", "version"}, ver, false, "", 0},
		{"--store=", []string{"--store=/x", "version"}, ver, false, "", 0},
		{"-- then command", []string{"--", "version"}, ver, false, "", 0},
		{"help", []string{"help"}, overviewHead, true, "", 0},
		{"--help", []string{"--help"}, overviewHead, true, "", 0},
		{"-h", []string{"-h"}, overviewHead, true, "", 0},
		{"no command", nil, "", false,
			"memex: missing command\nmemex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]\nRun: memex help\n", 2},
		{"unknown command, near", []string{"serch"}, "", false,
			"memex: unknown command \"serch\"; did you mean \"search\"? Run: memex help topics\n", 2},
		{"unknown command, far", []string{"xyzzyplugh"}, "", false,
			"memex: unknown command \"xyzzyplugh\". Run: memex help topics\n", 2},
		{"unknown global option", []string{"--stor", "x"}, "", false,
			"memex: unknown option --stor (did you mean --store?)\nmemex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]\nRun: memex help\n", 2},
		{"store value starting with -", []string{"--store", "-x", "version"}, "", false,
			"memex: value for --store starts with \"-\"; write --store=-x\nmemex [GLOBAL OPTIONS] COMMAND [OPTIONS] [ARGS]\nRun: memex help\n", 2},
		{"version with an argument", []string{"version", "x"}, "", false,
			"memex: unexpected argument \"x\"\nmemex version\nRun: memex help version\n", 2},
		{"version with an option", []string{"version", "--json"}, "", false,
			"memex: unknown option --json\nmemex version\nRun: memex help version\n", 2},
		{"not implemented", []string{"search", "nix"}, "", false,
			"memex: command \"search\" is not implemented yet\nmemex search QUERY… [--type T]… [--tag X]… [--repo R]… [--host H]… [--all] [--limit N] [--json]\nRun: memex help search\n", 2},
		{"not implemented, multi-line usage", []string{"init"}, "", false,
			"memex: command \"init\" is not implemented yet\nmemex init PATH [--name NAME]\nmemex init --pointers DIR [--force]\nRun: memex help init\n", 2},
		{"help for a command without a page", []string{"help", "search"}, "", false,
			"memex: no help for \"search\". Run: memex help topics\n", 2},
		{"--help on a command without a page", []string{"capture", "--help"}, "", false,
			"memex: no help for \"capture\". Run: memex help topics\n", 2},
		{"help --all", []string{"help", "--all"}, "", false,
			"memex: option --all is not implemented yet\nmemex help [COMMAND|TOPIC] [--all] [--json]\nRun: memex help help\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, errOut, code := invoke(c.args...)
			if code != c.code {
				t.Errorf("exit %d, want %d", code, c.code)
			}
			if c.prefix {
				if len(out) < len(c.stdout) || out[:len(c.stdout)] != c.stdout {
					t.Errorf("stdout %q, want prefix %q", out, c.stdout)
				}
			} else if out != c.stdout {
				t.Errorf("stdout %q, want %q", out, c.stdout)
			}
			if errOut != c.stderr {
				t.Errorf("stderr %q, want %q", errOut, c.stderr)
			}
		})
	}
}

func TestPanicExits70(t *testing.T) {
	saved := dispatchFunc
	defer func() { dispatchFunc = saved }()
	dispatchFunc = func([]string, io.Writer, io.Writer) (int, *usageError) { panic("boom") }
	_, errOut, code := invoke("version")
	if code != 70 || errOut != "memex: internal error: boom\n" {
		t.Errorf("got exit %d, stderr %q", code, errOut)
	}
}

func TestParseArgs(t *testing.T) {
	check, _ := spec.LookupCommand("check")
	newCmd, _ := spec.LookupCommand("new")
	save, _ := spec.LookupCommand("save")
	cases := []struct {
		cmd     *spec.Command
		argv    []string
		want    string // fmt of values and positionals, or the problem
		problem bool
	}{
		{check, []string{"--strict", "--limit", "3", "--json"}, "map[--json:[] --limit:[3] --strict:[]] []", false},
		{check, []string{"--limit=3", "--store", "/s"}, "map[--limit:[3] --store:[/s]] []", false},
		{check, []string{"--limit=-3"}, "map[--limit:[-3]] []", false},
		{check, []string{"--limit", "-3"}, `value for --limit starts with "-"; write --limit=-3`, true},
		{check, []string{"--limit"}, "missing value for --limit", true},
		{check, []string{"--limit=1", "--limit=2"}, "option --limit given more than once", true},
		{check, []string{"--json=yes"}, "option --json takes no value", true},
		{check, []string{"--stagd"}, "unknown option --stagd (did you mean --staged?)", true},
		{check, []string{"-x"}, "unknown option -x", true},
		{check, []string{"--store="}, "empty value for --store", true},
		{check, []string{"pos"}, `unexpected argument "pos"`, true},
		{check, []string{"--", "--json"}, `unexpected argument "--json"`, true},
		{newCmd, []string{"rule", "--tag", "a", "--title=-x", "--tag=b", "--body", "-"},
			"map[--body:[-] --tag:[a b] --title:[-x]] [rule]", false},
		{newCmd, []string{"--title", "-x", "rule"}, `value for --title starts with "-"; write --title=-x`, true},
		{save, []string{"-m", "msg", "a.md", "--", "-b.md"}, "map[-m:[msg]] [a.md -b.md]", false},
		{save, []string{"--m", "msg"}, "unknown option --m (did you mean -m?)", true},
	}
	for _, c := range cases {
		a, problem := parseArgs(c.cmd, c.argv)
		got := problem
		if problem == "" {
			got = fmt.Sprint(a.values, " ", a.positionals)
			got = strings.Replace(got, "  ", " ", 1)
		}
		if got != c.want || (problem != "") != c.problem {
			t.Errorf("%s %q:\n got %s\nwant %s", c.cmd.Name, c.argv, got, c.want)
		}
	}
}

func TestStoreAnywhere(t *testing.T) {
	cases := []struct {
		args   []string
		stderr string
	}{
		{[]string{"--store=", "version"}, "memex: empty value for --store\n"},
		{[]string{"--store=/a", "check", "--store=/b"}, "memex: option --store given more than once\n"},
		{[]string{"check", "--store=/nonexistent/x"}, "memex: store /nonexistent/x: no such file or directory\n"},
		{[]string{"check", "--store", "relative"}, "memex: store path \"relative\" is relative; give an absolute path\n"},
	}
	for _, c := range cases {
		_, errOut, code := invoke(c.args...)
		if code != 2 || !strings.HasPrefix(errOut, c.stderr) {
			t.Errorf("%q: exit %d, stderr %q", c.args, code, errOut)
		}
	}
}
