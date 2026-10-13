package slug

import (
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	for title, want := range map[string]string{
		"Bare rebuild leaves the closure uncached": "bare-rebuild-leaves-the-closure-uncached",
		"  --Hello, World!--  ":                    "hello-world",
		"Café crème":                               "cafe-creme",
		"ﬁle ①":                                    "file-1",
		"Straße":                                   "stra-e",
		"日本語":                                      "entry",
		"":                                         "entry",
		"ÅNGSTRÖM 2.0":                             "angstrom-2-0",
		"a_b":                                      "a-b",
	} {
		if got := Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestID(t *testing.T) {
	if got := ID("2026-01-15", "Hello"); got != "2026-01-15-hello" {
		t.Errorf("ID = %q", got)
	}
	long := strings.Repeat("word ", 40)
	id := ID("2026-01-15", long)
	// 111 characters, trailing "-" stripped, then "-" and 8 hex digits.
	if !ValidID(id) || len(id) > MaxIDLength {
		t.Fatalf("long id %q (%d) is not valid", id, len(id))
	}
	if !strings.HasPrefix(id, "2026-01-15-word-word") || id[len(id)-9] != '-' {
		t.Errorf("long id %q", id)
	}
	// A cut that ends on "-" loses it before the hash is appended.
	id = ID("2026-01-15", strings.Repeat("abcd ", 30))
	if strings.Contains(id, "--") || len(id) != 110+9 {
		t.Errorf("cut on a dash: %q (%d)", id, len(id))
	}
}

func TestValidID(t *testing.T) {
	for s, want := range map[string]bool{
		"2026-01-15-a":                           true,
		"2026-01-15-a-b-1":                       true,
		"2026-01-15":                             false,
		"2026-01-15-":                            false,
		"2026-01-15-a--b":                        false,
		"2026-01-15-A":                           false,
		"26-01-15-a":                             false,
		"2026-01-15-" + strings.Repeat("a", 109): true,
		"2026-01-15-" + strings.Repeat("a", 110): false,
	} {
		if ValidID(s) != want {
			t.Errorf("ValidID(%q) = %v", s, !want)
		}
	}
}
