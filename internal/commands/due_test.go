package commands

import (
	"testing"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
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
		// The hint is a guess from the wording (SPEC §11.18): these two name dates of the past
		// and are hinted all the same, which is why memex due labels hints as guesses and the
		// suggested memex set line as one to run only after reading the entry.
		{"2026-01-15 incident follow-up", "", "2026-01-15"},
		{"Check the migration", "The migration was completed by 2026-01-15.", "2026-01-15"},
	}
	for _, c := range cases {
		en := &store.Entry{Fields: entry.Fields{"title": c.title}, Body: c.body}
		if got := textDate(en); got != c.want {
			t.Errorf("textDate(%q, %q) = %q, want %q", c.title, c.body, got, c.want)
		}
	}
}

// dueProblem is what memex due skips (SPEC §11.18, review finding 4): every field it reads
// is checked, and due and due_when need store format 2.
func TestDueProblem(t *testing.T) {
	task := &spec.Types[4]
	cases := []struct {
		fields entry.Fields
		format int64
		want   string
	}{
		{entry.Fields{"title": "T", "status": "open"}, 2, ""},
		{entry.Fields{"title": "T"}, 2, `missing required field "status"`},
		{entry.Fields{"title": "T", "status": "opne"}, 2, "status is not valid"},
		{entry.Fields{"title": "T", "status": entry.Invalid{}}, 2, "status is not valid"},
		{entry.Fields{"status": "open"}, 2, `missing required field "title"`},
		{entry.Fields{"title": entry.Invalid{}, "status": "open"}, 2, "title is not valid"},
		{entry.Fields{"title": "T", "status": "open", "summary": entry.Invalid{}}, 2, "summary is not valid"},
		{entry.Fields{"title": "T", "status": "open", "due": entry.Invalid{}}, 2, "due is not valid"},
		{entry.Fields{"title": "T", "status": "open", "due_when": "x"}, 1, "due_when needs store format 2"},
		{entry.Fields{"title": "T\x1b[31m", "status": "open"}, 2, "title " + entry.ControlMessage},
		{entry.Fields{"title": "T\x1b[31m", "status": "open"}, 1, ""},
	}
	for _, c := range cases {
		en := &store.Entry{File: store.File{Type: task}, Fields: c.fields}
		if got := dueProblem(en, c.format); got != c.want {
			t.Errorf("dueProblem(%v, %d) = %q, want %q", c.fields, c.format, got, c.want)
		}
	}
}

// migratedManifest keeps every other line and adds or removes stale_days (SPEC §11.19).
func TestMigratedManifest(t *testing.T) {
	cases := []struct {
		in   string
		to   int64
		want string
	}{
		{"format = 1\nname = \"x\"\n", 2, "format = 2\nname = \"x\"\nstale_days = 14\n"},
		{"# a comment\nformat=1\nname = \"x\"", 2, "# a comment\nformat = 2\nname = \"x\"\nstale_days = 14\n"},
		{"format = 2\nstale_days = 7\nname = \"x\"\n", 1, "format = 1\nname = \"x\"\n"},
		{"format = 1\nstale_days = 7\n", 2, "format = 2\nstale_days = 7\n"},
	}
	for _, c := range cases {
		if got := string(migratedManifest([]byte(c.in), c.to)); got != c.want {
			t.Errorf("migratedManifest(%q, %d) = %q, want %q", c.in, c.to, got, c.want)
		}
	}
}
