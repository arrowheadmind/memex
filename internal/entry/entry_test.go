package entry

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/arrowheadmind/memex/internal/spec"
)

func codes(ps []Problem) string {
	var cs []string
	for _, p := range ps {
		cs = append(cs, p.Code+" "+p.Message)
	}
	return strings.Join(cs, "\n")
}

func TestSplitFile(t *testing.T) {
	s, p := SplitFile("+++\ntitle = \"x\"\n+++\n\nbody\n")
	if p != nil || s.Frontmatter != "title = \"x\"\n" || s.BodyRegion != "\nbody\n" || s.Body != "body\n" {
		t.Fatalf("%+v %v", s, p)
	}
	s, p = SplitFile("+++\n+++")
	if p != nil || s.Frontmatter != "" || s.BodyRegion != "" {
		t.Fatalf("no body region: %+v %v", s, p)
	}
	s, p = SplitFile("+++\na = 1\n+++\nbody")
	if p != nil || s.Body != "body" {
		t.Fatalf("no blank line: %+v %v", s, p)
	}
	for _, bad := range []string{"", "title = 1\n", "+++ \n+++\n", "+++\na = 1\n", "+++\na = 1\n+++ \n"} {
		if _, p := SplitFile(bad); p == nil || p.Code != "E-FRONTMATTER" {
			t.Errorf("%q: %v", bad, p)
		}
	}
}

func TestCheckEncoding(t *testing.T) {
	if ps := CheckEncoding([]byte("+++\n")); len(ps) != 0 {
		t.Errorf("clean: %v", ps)
	}
	if ps := CheckEncoding([]byte("\xef\xbb\xbf+++\r\n\xff")); len(ps) != 3 {
		t.Errorf("all three: %v", ps)
	}
}

func parse(t *testing.T, typ, front string) (Fields, []Problem) {
	t.Helper()
	return ParseFields(&Split{Frontmatter: front, FrontLine: 2}, spec.LookupType(typ), "2026-01-15-x")
}

func TestParseFields(t *testing.T) {
	front := "title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\nsummary = \"S\"\n" +
		"tags = [\"b\", \"a\", \"a\"]\nkeywords = []\nalways = true\nreview_days = 1\n"
	f, ps := parse(t, "rule", front)
	if len(ps) != 1 || ps[0].Code != "E-FIELD-UNKNOWN" || !strings.Contains(ps[0].Message, `"review_days" is not allowed on a rule`) {
		t.Errorf("problems: %s", codes(ps))
	}
	if f.String("title") != "T" || len(f.Strings("tags")) != 3 || f["always"] != true {
		t.Errorf("fields: %v", f)
	}
	cases := map[string]string{
		"title = \"T\"\nstatus = \"actve\"\ncreated = 2026-01-15\n":                         `E-STATUS status "actve" is not valid for a rule (did you mean "active"?); valid: active, superseded, retired`,
		"title = \"T\"\nstatus = \"active\"\n":                                              `E-FIELD-MISSING missing required field "created"`,
		"title = \"T\"\nstatus = \"active\"\ncreated = \"2026-01-15\"\n":                    "E-FIELD-VALUE created must be a date written YYYY-MM-DD, unquoted",
		"title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15T00:00:00Z\n":              "E-FIELD-VALUE created must be a date written YYYY-MM-DD, unquoted",
		"title = \"\"\nstatus = \"active\"\ncreated = 2026-01-15\n":                         "E-FIELD-VALUE title must be 1 to 300 characters on one line",
		"title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\ntags = [\"A\"]\n":        `E-FIELD-VALUE tags element "A" does not match ^[a-z0-9][a-z0-9-]*$`,
		"title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\ntags = [1]\n":            "E-FIELD-VALUE tags must be an array of strings",
		"title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\ntitel = \"x\"\n":         `E-FIELD-UNKNOWN unknown field "titel" (did you mean "title"?)`,
		"title = \"T\"\nstatus = \"superseded\"\ncreated = 2026-01-15\n":                    "E-SUPERSEDED status is superseded but superseded_by is missing; add it: memex set 2026-01-15-x superseded_by=<new>",
		"title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\nsuperseded_by = \"x\"\n": `E-SUPERSEDED superseded_by is set but status is "active"; either memex set 2026-01-15-x status=superseded, or memex set 2026-01-15-x superseded_by=`,
		"title = \"T\"\nstatus = \"active\"\n\ncreated = 2026-01-15\ntitle = \"U\"\n":       "E-FRONTMATTER invalid TOML in the frontmatter at line 6",
	}
	for front, want := range cases {
		_, ps := parse(t, "rule", front)
		if got := codes(ps); got != want {
			t.Errorf("%q:\n got %s\nwant %s", front, got, want)
		}
	}
	_, ps = parse(t, "reference", "title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\nreview_days = 0\n")
	if got := codes(ps); got != "E-FIELD-VALUE review_days must be between 1 and 36500\nE-FIELD-MISSING missing required field \"verified\"" {
		t.Errorf("reference: %s", got)
	}
	_, ps = parse(t, "journal", "title = \"T\"\nstatus = \"raw\"\ncreated = 2026-01-15\nbody_sha256 = \"ABC\"\n")
	if got := codes(ps); got != "E-FIELD-VALUE body_sha256 must be 64 lowercase hex digits" {
		t.Errorf("journal: %s", got)
	}
}

func TestRender(t *testing.T) {
	front := "keywords = [\"b\", \"a\", \"a\", \"\u00e9\"]\nsources = [\"z\", \"a\"]\ntags = []\ncreated = 2026-01-15\n" +
		"status = \"active\"\ntitle = \"q\\\"b\\\\s\\u0001\\u007f\\t\u2028\"\nalways = false\n"
	f, ps := parse(t, "rule", front)
	if len(ps) != 0 {
		t.Fatal(codes(ps))
	}
	want := "+++\ntitle = \"q\\\"b\\\\s\\u0001\\u007F\\t\u2028\"\nstatus = \"active\"\ncreated = 2026-01-15\n" +
		"keywords = [\"a\", \"b\", \"\u00e9\"]\nsources = [\"z\", \"a\"]\n+++\n\nbody\n"
	if got := Render(f, "body\n"); got != want {
		t.Errorf("Render:\n%s\nwant\n%s", got, want)
	}
	f, _ = parse(t, "reference", "title = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\nverified = 2026-01-01\nreview_days = 30\nalways = true\n")
	delete(f, "always")
	if got := Render(f, ""); got != "+++\ntitle = \"T\"\nstatus = \"active\"\ncreated = 2026-01-15\nverified = 2026-01-01\nreview_days = 30\n+++\n\n" {
		t.Errorf("reference: %q", got)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"a\r\nb\rc  \n\n\t": "a\nb\nc\n",
		"":                  "\n",
		"x\u00a0\n":         "x\n",
		"  lead":            "  lead\n",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFenced(t *testing.T) {
	lines := []string{"a", "```sh", "x", "``", "````", "b", "   ~~~", "~~~~ ", "c", "    ```", "d", "~~~", "e"}
	want := []bool{false, true, true, true, true, false, true, true, false, false, false, true, true}
	got := Fenced(lines)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d %q: fenced %v, want %v", i, lines[i], got[i], want[i])
		}
	}
}

func TestContentMeasure(t *testing.T) {
	for body, want := range map[string]int{
		"":                         0,
		"# Heading\n## Two\n":      0,
		"#nospace\n":               8,
		"    # indented code\n":    13,
		"```\n# comment\n```\n":    14,
		"ab c\u00a0d\n####### x\n": 12,
	} {
		if got := ContentMeasure(body); got != want {
			t.Errorf("ContentMeasure(%q) = %d, want %d", body, got, want)
		}
	}
}

func TestLocalDateLocation(t *testing.T) {
	var m map[string]any
	if _, err := toml.Decode("d = 2026-01-15\nt = 2026-01-15T00:00:00\nz = 2026-01-15T00:00:00Z\n", &m); err != nil {
		t.Fatal(err)
	}
	if tomlKindOf(m["d"]) != "Local Date" || tomlKindOf(m["t"]) != "Datetime" || tomlKindOf(m["z"]) != "Datetime" {
		t.Errorf("BurntSushi/toml no longer marks Local Date by location: %v %v %v", m["d"], m["t"], m["z"])
	}
}
