package importv3

import "testing"

func TestMapBody(t *testing.T) {
	m := map[string]string{
		"2026-01-11-a":   "2026-01-11-a-2",
		"2026-01-11-a-2": "2026-01-11-a",
		"2026-01-12-b":   "2026-01-12-b",
	}
	in := "See [[2026-01-11-a]] and [[2026-01-11-a-2|the rule]] and [[2026-01-12-b]].\n" +
		"In code: `[[2026-01-11-a]]` stays.\n" +
		"```\n[[2026-01-11-a]]\n```\n" +
		"- files/2026-01-11-a/x.tsv\n"
	want := "See [[2026-01-11-a-2]] and [[2026-01-11-a|the rule]] and [[2026-01-12-b]].\n" +
		"In code: `[[2026-01-11-a]]` stays.\n" +
		"```\n[[2026-01-11-a]]\n```\n" +
		"- files/2026-01-11-a-2/x.tsv\n"
	if got := mapBody(in, m); got != want {
		t.Errorf("mapBody:\n%s\nwant:\n%s", got, want)
	}
	if got := mapBody("no links\n", m); got != "no links\n" {
		t.Errorf("unchanged body changed: %q", got)
	}
}

func TestFreeID(t *testing.T) {
	used := map[string]bool{"2026-01-11-a": true, "2026-01-11-a-2": true}
	if got := freeID("2026-01-11-a", used); got != "2026-01-11-a-3" {
		t.Errorf("freeID = %s", got)
	}
}
