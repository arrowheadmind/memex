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

func TestBacklogStatus(t *testing.T) {
	cases := map[string]string{
		"✓ CLOSED 2026-10-07 — secrets deleted": "done",
		"⏸ RESUME HERE — state":                 "blocked",
		"Item SUPERSEDED by other":              "dropped",
		"~~Struck item~~":                       "dropped",
		"Plain item":                            "",
		"Resolved the fixed thing":              "",
		"retired `es.` blog was closed":         "",
		"blocked the mesh":                      "",
		"Retired-service facts":                 "",
	}
	for in, want := range cases {
		if got := backlogStatus(in); got != want {
			t.Errorf("backlogStatus(%q) = %q, want %q", in, got, want)
		}
	}
	rest, removed := removeMarkers("📋 ✓ CLOSED 2026-10-07 — secrets DONE")
	if rest != "📋 2026-10-07 — secrets" || !reflect.DeepEqual(removed, []string{"✓", "CLOSED", "DONE"}) {
		t.Errorf("removeMarkers = %q, %q", rest, removed)
	}
}

func TestMarkerTags(t *testing.T) {
	got := markerTags("This is STALE and was resolved; no longer applies")
	want := []string{"marker-stale", "marker-resolved", "marker-no-longer"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("markerTags = %v, want %v", got, want)
	}
	if markerTags("installed, closedown") != nil {
		t.Errorf("markerTags matched inside words")
	}
}

func TestCompareText(t *testing.T) {
	if compareText("**a** | b \\| `c`") != "abc" {
		t.Errorf("compareText = %q", compareText("**a** | b \\| `c`"))
	}
}

func TestParseSourceArg(t *testing.T) {
	p, r := ParseSourceArg("/x/y@abc")
	if p != "/x/y" || r != "abc" {
		t.Errorf("ParseSourceArg = %q, %q", p, r)
	}
	p, r = ParseSourceArg("/x/y")
	if p != "/x/y" || r != "" {
		t.Errorf("ParseSourceArg = %q, %q", p, r)
	}
}

func TestIDFor(t *testing.T) {
	cases := []struct{ date, title, want string }{
		{"2026-10-12", "2026-10-12 — rotate the password", "2026-10-12-rotate-the-password"},
		{"2026-10-12", "2026-10-13 — vps-a MySQL (filed 2026-10-12)", "2026-10-12-vps-a-mysql-filed-2026-10-12"},
		{"2026-10-12", "Plain title", "2026-10-12-plain-title"},
		{"2026-10-12", "2026-10-12", "2026-10-12-entry"},
	}
	for _, c := range cases {
		if got := idFor(c.date, c.title); got != c.want {
			t.Errorf("idFor(%q, %q) = %q, want %q", c.date, c.title, got, c.want)
		}
	}
}
