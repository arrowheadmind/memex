package importv3

import (
	"reflect"
	"testing"
)

func TestSplitCells(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"| a | b | c |", []string{"a", "b", "c"}},
		{`| a \| b | c |`, []string{`a \| b`, "c"}},
		{"| a | b | c | d |", []string{"a", "b", "c", "d"}},
		{"|a|b|", []string{"a", "b"}},
	}
	for _, c := range cases {
		if got := splitCells(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitCells(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHeadingDate(t *testing.T) {
	cases := []struct {
		in, title, date, removed string
	}{
		{"Fix the thing (2026-09-01)", "Fix the thing", "2026-09-01", " (2026-09-01)"},
		{"Attic push broke, found 2026-09-11", "Attic push broke", "2026-09-11", ", found 2026-09-11"},
		{"RETIRED 2026-09-13 — policy", "RETIRED 2026-09-13 — policy", "2026-09-13", ""},
		{"Attic push, found 2026-09-11, resolved 2026-09-12", "Attic push, found 2026-09-11, resolved 2026-09-12", "2026-09-11", ""},
		{"Network — 2026-01-05", "Network", "2026-01-05", " — 2026-01-05"},
		{"No date here", "No date here", "", ""},
	}
	for _, c := range cases {
		title, date, removed := headingDate(c.in)
		if title != c.title || date != c.date || removed != c.removed {
			t.Errorf("headingDate(%q) = %q, %q, %q; want %q, %q, %q", c.in, title, date, removed, c.title, c.date, c.removed)
		}
	}
}

func TestStripPrefix(t *testing.T) {
	cases := []struct{ in, rest, tag, removed, date string }{
		{"Mistake: did X", "did X", "mistake", "Mistake:", ""},
		{"Known Noise: Y", "Y", "noise", "Known Noise:", ""},
		{"Mistake / technique: Z", "Z", "technique", "Mistake / technique:", ""},
		{"Historical mistake (superseded 2026-08-26): W", "W", "historical", "Historical mistake (superseded 2026-08-26): ", "2026-08-26"},
		{"Historical mistake: V", "V", "historical", "Historical mistake: ", ""},
		{"Plain", "Plain", "", "", ""},
	}
	for _, c := range cases {
		rest, tag, removed, date := stripPrefix(c.in)
		if rest != c.rest || tag != c.tag || removed != c.removed || date != c.date {
			t.Errorf("stripPrefix(%q) = %q, %q, %q, %q; want %q, %q, %q, %q", c.in, rest, tag, removed, date, c.rest, c.tag, c.removed, c.date)
		}
	}
}

func TestStatusValue(t *testing.T) {
	cases := map[string]string{
		"complete — 2026-07-06": "complete",
		"PLANNED":               "planned",
		"- [x] Colors changed":  "",
		"raw":                   "raw",
		"":                      "",
		"Accepted.":             "accepted",
	}
	for in, want := range cases {
		if got := statusValue(in); got != want {
			t.Errorf("statusValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTagify(t *testing.T) {
	cases := map[string]string{"gRPC": "grpc", "deploy.sh": "deploy-sh", " gcr.io ": "gcr-io", "---": "", "Nix OS": "nix-os"}
	for in, want := range cases {
		if got := tagify(in); got != want {
			t.Errorf("tagify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeadingMarker(t *testing.T) {
	cases := []struct{ in, status, rest, removed string }{
		{"Retired-service facts outlive the service, and `constraints.md` misleads — ✓ DONE 2026-09-14 (`brain@45d690b`); 0 `SUPERSEDED` markers remain",
			"done", "Retired-service facts outlive the service, and `constraints.md` misleads — 2026-09-14 (`brain@45d690b`); 0 `SUPERSEDED` markers remain", "✓ DONE"},
		{"✓ RESOLVED 2026-09-23 — (was BLOCKED) the two NAS boxes get a certificate", "done", "2026-09-23 — (was BLOCKED) the two NAS boxes get a certificate", "✓ RESOLVED"},
		{"nas-a strip-down: repo and docs done, host deletion BLOCKED and deliberately held", "", "nas-a strip-down: repo and docs done, host deletion BLOCKED and deliberately held", ""},
		{"Old plan for the mesh MOOT", "dropped", "Old plan for the mesh", "MOOT"},
		{"2026-10-07 — RETIRED the queue that waits", "dropped", "2026-10-07 — the queue that waits", "RETIRED"},
		{"⏸ RESUME HERE — state", "blocked", "RESUME HERE — state", "⏸"},
		{"~~Struck item~~", "dropped", "Struck item", "~~"},
		{"retired `es.` blog was closed", "", "retired `es.` blog was closed", ""},
		{"📋 OPERATOR TO-DO, index", "", "📋 OPERATOR TO-DO, index", ""},
	}
	for _, c := range cases {
		st, rest, removed := headingMarker(c.in)
		if st != c.status || rest != c.rest || removed != c.removed {
			t.Errorf("headingMarker(%q) = %q, %q, %q; want %q, %q, %q", c.in, st, rest, removed, c.status, c.rest, c.removed)
		}
	}
}

func TestSearchKey(t *testing.T) {
	cases := map[string]string{
		"### ✓ CLOSED 2026-10-07 — three plaintext secret files":                              "— three plaintext secret files",
		"| **Short** | row \\| x | - |":                                                       "**Short** | row \\| x | -",
		"### Item SUPERSEDED by the big plan":                                                 "by the big plan",
		"## Mistake: a very long heading that keeps going well past the sixty code point cut": "Mistake: a very long heading that keeps going well past the",
	}
	for in, want := range cases {
		if got := searchKey(in); got != want {
			t.Errorf("searchKey(%q) = %q, want %q", in, got, want)
		}
	}
}
