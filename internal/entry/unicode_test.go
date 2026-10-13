package entry

import (
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TestUnicodeTablesAgree asserts that the stdlib tables (general categories, simple lowercase)
// and x/text's normalization tables are the same Unicode version, the one SPEC §5.1 names.
func TestUnicodeTablesAgree(t *testing.T) {
	if unicode.Version != norm.Version {
		t.Fatalf("unicode.Version %s, norm.Version %s: pin an x/text release with matching tables", unicode.Version, norm.Version)
	}
	if unicode.Version != "17.0.0" {
		t.Fatalf("unicode.Version %s, SPEC §5.1 requires 17.0.0", unicode.Version)
	}
}
