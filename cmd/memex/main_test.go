package main

import (
	"bytes"
	"io"
	"testing"
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
	dispatchFunc = func([]string, io.Writer) (int, *usageError) { panic("boom") }
	_, errOut, code := invoke("version")
	if code != 70 || errOut != "memex: internal error: boom\n" {
		t.Errorf("got exit %d, stderr %q", code, errOut)
	}
}
