package commands

import (
	"testing"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/store"
)

// textDate is the "date in the text" of SPEC §11.18: a title that begins with a date, or a
// deadline word right before a date, in the title or the body outside fenced code.
func TestTextDate(t *testing.T) {
	cases := []struct {
		title, body, want string
	}{
		{"2026-11-26: 90-day review of the dumps", "", "2026-11-26"},
		{"2026-11-15 retest the collector", "", "2026-11-15"},
		{"2026-11-15x is not a date", "", ""},
		{"20261115 is not a date", "", ""},
		{"2026-02-30: no such day", "", ""},
		{"Revisit on or after 2026-11-12: memory", "", "2026-11-12"},
		{"ON OR AFTER 2026-10-22: judge the test", "", "2026-10-22"},
		{"Rotate the key", "Rotate it by 2026-11-21 at the latest.", "2026-11-21"},
		{"Rotate the key", "Rotate it by **2026-11-21** at the latest.", "2026-11-21"},
		{"Rotate the key", "Due: 2026-03-01, as agreed.", "2026-03-01"},
		{"Rotate the key", "**Open, deadline 2026-11-04 06:30Z:** the window.", "2026-11-04"},
		{"Rotate the key", "no later than\t2026-12-01", "2026-12-01"},
		{"Rotate the key", "Deleted before 2026-10-28; valid until 2026-11-06; re-check on 2026-11-09.", ""},
		{"Rotate the key", "The standby 2026-01-20 window; nearby2026-01-21.", ""},
		{"Rotate the key", "due 2026-02-30, then by 2026-03-02.", "2026-03-02"},
		{"Rotate the key", "due 2026-01-200 is not a date", ""},
		{"Rotate the key", "```\ndue 2026-01-20\n```\n", ""},
		{"Rotate the key", "by 11-26, the short form", ""},
		{"Due 2026-04-01 in the title", "by 2026-05-01", "2026-04-01"},
		{"A plain title", "first by 2026-05-01, then due 2026-04-01", "2026-05-01"},
		{"Café by 2026-05-01", "", "2026-05-01"},
		{"Éby 2026-05-01", "", ""},
		{"Rotate the key", "## Deadline 2026-01-15\n\nMoved.", ""},
		{"Rotate the key", "## Due 2026-01-15 for the vendor\n", "2026-01-15"},
		{"Rotate the key", "Run `due 2026-01-20` to see it.", ""},
		{"Rotate the key", "Run `x` by 2026-01-20.", "2026-01-20"},
	}
	for _, c := range cases {
		en := &store.Entry{Fields: entry.Fields{"title": c.title}, Body: c.body}
		if got := textDate(en); got != c.want {
			t.Errorf("textDate(%q, %q) = %q, want %q", c.title, c.body, got, c.want)
		}
	}
}
