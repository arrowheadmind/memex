package check

import (
	"regexp"
	"strings"

	"github.com/arrowheadmind/memex/internal/entry"
)

// allowSecret is the body line that suppresses W-SECRET (SPEC §7).
const allowSecret = "<!-- memex: allow-secret -->"

var (
	pemKey     = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	assignment = regexp.MustCompile(`(password|token|secret)=("[^"]*"|'[^']*'|\S*)`)
	base64Run  = regexp.MustCompile(`[A-Za-z0-9+/]{40,}`)
	// secretWord must precede a base64 run on the same line for the run to count (SPEC §7);
	// without it the pattern matched every nix cache key and content hash.
	secretWord = regexp.MustCompile(`(?i)token|secret|password|passwd|bearer|authorization|api_key|api-key|apikey|private`)
)

type secretLine struct {
	line int
	what string
}

// secrets returns the body lines that look like credentials (SPEC §7, W-SECRET), unless the
// body holds the allow-secret line.
func secrets(body string) []secretLine {
	lines := entry.Lines(body)
	for _, l := range lines {
		if l == allowSecret {
			return nil
		}
	}
	var out []secretLine
	for i, l := range lines {
		what := ""
		switch {
		case pemKey.MatchString(l):
			what = "a private key header"
		case hasSecretAssignment(l):
			what = "a password, token or secret assignment"
		case hasMixedRun(l):
			what = "a run of 40 or more base64 characters"
		}
		if what != "" {
			out = append(out, secretLine{i + 1, what})
		}
	}
	return out
}

// hasMixedRun reports a run of 40 or more base64 characters holding at least one uppercase
// letter, one lowercase letter and one digit (which excludes hex digests and nix store hashes
// by construction), preceded on the same line by a word that says it is a credential.
func hasMixedRun(line string) bool {
	for _, loc := range base64Run.FindAllStringIndex(line, -1) {
		run := line[loc[0]:loc[1]]
		if strings.ContainsAny(run, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") &&
			strings.ContainsAny(run, "abcdefghijklmnopqrstuvwxyz") &&
			strings.ContainsAny(run, "0123456789") &&
			secretWord.MatchString(line[:loc[0]]) {
			return true
		}
	}
	return false
}

func hasSecretAssignment(line string) bool {
	for _, m := range assignment.FindAllStringSubmatch(line, -1) {
		if !isPlaceholder(m[2]) {
			return true
		}
	}
	return false
}

// isPlaceholder reports whether an assigned value is a placeholder rather than a credential:
// empty (after removing quotes), a variable or template reference (starting with $, <, { or
// %), or a mask made only of *, x, X, . and ….
func isPlaceholder(v string) bool {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	if v == "" || strings.ContainsAny(v[:1], "$<{%") {
		return true
	}
	return strings.Trim(v, "*xX.…") == ""
}
