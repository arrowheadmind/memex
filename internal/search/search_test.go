package search

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

func TestTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Café au lait", []string{"cafe", "au", "lait"}},
		{"Cafés", []string{"cafes"}}, // a combining mark is removed, not a separator
		{"ﬁle-system", []string{"file", "system"}},
		{"हिन्दी भाषा", []string{"हिनदी", "भाषा"}}, // the virama (Mn) goes, the vowel signs (Mc) stay
		{"nix_build 2026-01-15", []string{"nix", "build", "2026", "01", "15"}},
		{"ÉCOLE", []string{"ecole"}},
		{"", nil},
		{"--- !!", nil},
	}
	for _, c := range cases {
		if got := Tokens(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Tokens(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseQuery(t *testing.T) {
	cases := []struct {
		in   string
		want []string // Item.String()
	}{
		{`cache rebuild "nix build"`, []string{"cache", "rebuild", `"nix build"`}},
		{`"nix build" cache`, []string{`"nix build"`, "cache"}},
		{`cache Cache CACHE`, []string{"cache"}},
		{`"cache" cache`, []string{`"cache"`, "cache"}},
		{`"a b" "A B"`, []string{`"a b"`}},
		{`nix "build`, []string{"nix", "build"}},
		{`"a b" "c`, []string{`"a b"`, "c"}},
		{`""`, nil},
		{`x""y`, []string{"x", "y"}},
	}
	for _, c := range cases {
		q := ParseQuery(c.in)
		var got []string
		for _, it := range q.Items {
			got = append(got, it.String())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("ParseQuery(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrefix(t *testing.T) {
	cases := map[string]bool{"nix": false, "cach": true, "Café": true, "ﬁx": false, "ﬁxe": true}
	for in, want := range cases {
		it := ParseQuery(in).Items[0]
		if it.Prefix() != want {
			t.Errorf("%q: prefix %v, want %v", in, it.Prefix(), want)
		}
	}
	it := ParseQuery("cach").Items[0]
	if !it.matchToken("caches") || !it.matchToken("cach") || it.matchToken("cac") {
		t.Error("prefix term matching")
	}
	it = ParseQuery("nix").Items[0]
	if it.matchToken("nixos") || !it.matchToken("nix") {
		t.Error("short term matching")
	}
}

func TestPhraseHits(t *testing.T) {
	it := ParseQuery(`"a a"`).Items[0]
	if n := it.hits([]string{"a", "a", "a", "b", "a"}); n != 2 {
		t.Errorf("overlapping phrase hits = %d, want 2", n)
	}
}

func TestRound(t *testing.T) {
	cases := map[float64]float64{
		12.3456789012: 12.3456789,
		0.1234567895:  0.123456789, // the binary value is just below the half
		1.000000025:   1.00000003,  // the binary value is just above the half

		1: 1,
	}
	for in, want := range cases {
		if got := Round(in); got != want {
			t.Errorf("Round(%v) = %v, want %v", in, got, want)
		}
	}
}

func snippetOf(line, query string) string {
	sn := MakeSnippet(line+"\n", ParseQuery(query).Items)
	if sn == nil {
		return "<nil>"
	}
	return sn.String(true)
}

func TestSnippet(t *testing.T) {
	long := strings.Repeat("x", 100) + " cache " + strings.Repeat("y", 100)
	cases := []struct{ line, query, want string }{
		{"  the Café is open  ", "cafe", "the **Café** is open"},
		{"a ﬁle here", "file", "a **ﬁle** here"},
		{"Les Cafe\u0301s", "cafe", "Les **Cafe\u0301s**"}, // the mark stays in the run
		{"½ cup", "2", "**½** cup"},                        // ½ normalizes to the tokens 1 and 2
		{"run nix build now", `"nix build"`, "run **nix** **build** now"},
		{"nix then build", `"nix build"`, "<nil>"},
		{"caches and cache", "cach", "**caches** and **cache**"},
		// 207 code points, cache at 101: the window would start at 61, and is moved left to
		// 47 so that it ends at the line's end.
		{long, "cache", "…" + strings.Repeat("x", 53) + " **cache** " + strings.Repeat("y", 100)},
	}
	for _, c := range cases {
		if got := snippetOf(c.line, c.query); got != c.want {
			t.Errorf("snippet(%q, %q) =\n%q, want\n%q", c.line, c.query, got, c.want)
		}
	}
}

func TestSnippetWindow(t *testing.T) {
	// The match near the end: the window is moved left to end at the line's end.
	line := strings.Repeat("a ", 100) + "cache"
	got := snippetOf(line, "cache")
	if !strings.HasPrefix(got, "…") || strings.HasSuffix(got, "…") || !strings.HasSuffix(got, "**cache**") {
		t.Errorf("end window: %q", got)
	}
	if n := len([]rune(strings.ReplaceAll(strings.TrimPrefix(got, "…"), "**", ""))); n != SnippetWidth {
		t.Errorf("end window has %d code points", n)
	}
	// A matching token cut by the right edge is dropped from the window.
	line = strings.Repeat("b", 50) + " cache " + strings.Repeat("c", 110) + " cacheX tail of the line"
	got = snippetOf(line, "cache")
	want := "…" + strings.Repeat("b", 39) + " **cache** " + strings.Repeat("c", 110) + "…"
	if got != want {
		t.Errorf("right cut:\n%q, want\n%q", got, want)
	}
	// Fenced lines are skipped.
	sn := MakeSnippet("```\ncache\n```\nthe cache\n", ParseQuery("cache").Items)
	if sn == nil || sn.String(false) != "the cache" {
		t.Errorf("fenced: %v", sn)
	}
}

func mkEntry(t *testing.T, typ, id, status, title string, extra entry.Fields, body string) *store.Entry {
	t.Helper()
	ty := spec.LookupType(typ)
	f := entry.Fields{"title": title, "status": status}
	for k, v := range extra {
		f[k] = v
	}
	return &store.Entry{File: store.File{Path: ty.Directory + "/" + id + ".md", Type: ty, ID: id}, Fields: f, Body: body}
}

func TestScore(t *testing.T) {
	// Two documents; the arithmetic is the formula of SPEC §8.3 written out.
	a := mkEntry(t, "rule", "2026-01-10-a", "active", "Cache", nil, "one two three\n")
	b := mkEntry(t, "journal", "2026-01-10-b", "raw", "Other", nil, "cache four five six seven\n")
	c := NewCorpus([]*store.Entry{a, b})
	hits := c.Search(ParseQuery("cache").Items)
	if len(hits) != 2 || hits[0].Entry != a {
		t.Fatalf("hits %v", hits)
	}
	// N=2, n=2: idf = ln(1 + 0.5/2.5). |a|=4, |b|=6, avgdl=5.
	idf := math.Log(1 + 0.5/2.5)
	fa, fb := 10.0, 1.0
	sa := idf * fa * 2.2 / (fa + 1.2*(0.25+0.75*4/5)) * 1.5
	sb := idf * fb * 2.2 / (fb + 1.2*(0.25+0.75*6/5)) * 0.8
	if hits[0].Score != Round(sa) || hits[1].Score != Round(sb) {
		t.Errorf("scores %v %v, want %v %v", hits[0].Score, hits[1].Score, Round(sa), Round(sb))
	}
	if math.IsNaN(hits[0].Score) {
		t.Error("NaN")
	}
}
