// Package slug implements the ids of SPEC §5.4.
package slug

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MaxIDLength is the longest valid id, in bytes (ids are ASCII).
const MaxIDLength = 120

var idPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidID reports whether s matches the id pattern and length of SPEC §5.4. It does not check
// that the date is a real calendar date; E-CREATED does that through `created`.
func ValidID(s string) bool {
	return len(s) <= MaxIDLength && idPattern.MatchString(s)
}

// Slug turns a title into a slug: NFKD without Mn, simple lowercase, runs outside [a-z0-9]
// collapsed to "-", trimmed, and "entry" when empty.
func Slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFKD.String(title) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		r = unicode.ToLower(r)
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	if b.Len() == 0 {
		return "entry"
	}
	return b.String()
}

// ID returns the id for a title created on date (YYYY-MM-DD), with the length cap.
func ID(date, title string) string {
	id := date + "-" + Slug(title)
	if len(id) <= MaxIDLength {
		return id
	}
	sum := sha256.Sum256([]byte(title))
	return strings.TrimRight(id[:111], "-") + "-" + hex.EncodeToString(sum[:])[:8]
}
