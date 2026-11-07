package main

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

// The README's Status line and SPEC.md's tool version name the newest release of
// CHANGELOG.md, version and date. A release commit that sets the CHANGELOG heading but not
// those lines fails here: in scripts/check.sh, in CI, and in the nix build that release.yml
// runs for the tag (the README once fell three releases behind).
var (
	readmeStatus  = regexp.MustCompile(`(?m)^\*\*Status:\*\* (\S+) released (\S+);`)
	specVersion   = regexp.MustCompile(`(?m)^\*\*Store format version \d+\. Tool version: (\S+) \(released (\S+)\)\.\*\*$`)
	changelogHead = regexp.MustCompile(`(?m)^## (\S+) \((\S+)\)$`)
)

// docsNameNewestRelease returns "" when README.md and SPEC.md name the newest release of the
// changelog (its first `## X.Y.Z (YYYY-MM-DD)` heading), else the first difference.
func docsNameNewestRelease(readme, spec, changelog string) string {
	c := changelogHead.FindStringSubmatch(changelog)
	if c == nil {
		return "CHANGELOG.md has no '## X.Y.Z (YYYY-MM-DD)' heading"
	}
	for _, d := range []struct {
		file, line string
		re         *regexp.Regexp
		text       string
	}{
		{"README.md", "'**Status:** X.Y.Z released YYYY-MM-DD;'", readmeStatus, readme},
		{"SPEC.md", "'Tool version: X.Y.Z (released YYYY-MM-DD)'", specVersion, spec},
	} {
		m := d.re.FindStringSubmatch(d.text)
		if m == nil {
			return d.file + " has no " + d.line + " line"
		}
		if m[1] != c[1] || m[2] != c[2] {
			return fmt.Sprintf("%s names %s released %s; CHANGELOG.md's newest release is %s (%s)",
				d.file, m[1], m[2], c[1], c[2])
		}
	}
	return ""
}

func TestDocsNameNewestRelease(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if d := docsNameNewestRelease(read("README.md"), read("SPEC.md"), read("CHANGELOG.md")); d != "" {
		t.Fatal(d)
	}
}

func TestDocsNameNewestReleaseOtherAnswer(t *testing.T) {
	const changelog = "# Changelog\n\n## Unreleased\n\n## 0.5.1 (2026-11-04)\n\n### Changed\n\n## 0.5.0 (2026-11-03)\n"
	const spec = "# memex — specification\n\n**Store format version 1. Tool version: 0.5.1 (released 2026-11-04).**\n"
	readme := func(v, d string) string {
		return "# memex\n\n**Status:** " + v + " released " + d + "; the specification and the Go implementation are\ncomplete for that release.\n"
	}
	cases := []struct {
		name                    string
		readme, spec, changelog string
		want                    string
	}{
		{"the newest release", readme("0.5.1", "2026-11-04"), spec, changelog, ""},
		{"an older release", readme("0.2.0", "2026-10-25"), spec, changelog,
			"README.md names 0.2.0 released 2026-10-25; CHANGELOG.md's newest release is 0.5.1 (2026-11-04)"},
		{"the right version, another date", readme("0.5.1", "2026-11-03"), spec, changelog,
			"README.md names 0.5.1 released 2026-11-03; CHANGELOG.md's newest release is 0.5.1 (2026-11-04)"},
		{"no status line", "# memex\n\nStatus: 0.5.1\n", spec, changelog,
			"README.md has no '**Status:** X.Y.Z released YYYY-MM-DD;' line"},
		{"the spec behind", readme("0.5.1", "2026-11-04"),
			"**Store format version 1. Tool version: 0.5.0 (released 2026-11-03).**\n", changelog,
			"SPEC.md names 0.5.0 released 2026-11-03; CHANGELOG.md's newest release is 0.5.1 (2026-11-04)"},
		{"no release yet", readme("0.5.1", "2026-11-04"), spec, "# Changelog\n\n## Unreleased\n",
			"CHANGELOG.md has no '## X.Y.Z (YYYY-MM-DD)' heading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := docsNameNewestRelease(c.readme, c.spec, c.changelog); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
