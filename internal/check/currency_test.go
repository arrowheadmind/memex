package check

import "testing"

func TestNewSection(t *testing.T) {
	cases := []struct {
		newer, older, want string
		ok                 bool
	}{
		{"## A\n\nx\n", "## A\n\nx\n", "", false},
		{"## A\n\nx\n\n## B  \n", "## A\n", "## B  ", true},
		{"## A\n## A\n", "## A\n", "## A", true},
		{"## A\n\n```\n## B\n```\n", "## A\n", "", false},
		{"   # Top\n", "", "   # Top", true},
		{"#not a heading\n", "", "", false},
		{"## B\n", "## A\n## B\n", "", false}, // a section removed is not one added
	}
	for _, c := range cases {
		got, ok := newSection(c.newer, c.older)
		if got != c.want || ok != c.ok {
			t.Errorf("newSection(%q, %q) = %q, %v; want %q, %v", c.newer, c.older, got, ok, c.want, c.ok)
		}
	}
}

func TestHeadingName(t *testing.T) {
	for line, want := range map[string]string{
		"## Status 2026-01-12": "Status 2026-01-12",
		"   ### Closed \t":     "Closed",
		"#":                    "",
		"## Done ##":           "Done ##",
	} {
		if got := headingName(line); got != want {
			t.Errorf("headingName(%q) = %q, want %q", line, got, want)
		}
	}
}
