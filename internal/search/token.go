// Package search implements SPEC §8: the corpus, tokenization, query parsing, BM25 scoring,
// ranking, filters, snippets and the text and JSON output of search results.
package search

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// normalize applies steps 1 and 2 of SPEC §8.2: NFKD, removal of category Mn, then the
// simple lowercase mapping, one code point at a time.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// isTokenRune reports whether r is of general category L*, N* or M* (SPEC §8.2 step 3).
// After Mn removal only Mc and Me marks remain in normalized text; they keep scripts such as
// Devanagari whole.
func isTokenRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }

// Tokens tokenizes s by SPEC §8.2: the maximal runs of L*, N* or M* code points of the
// normalized text.
func Tokens(s string) []string {
	return strings.FieldsFunc(normalize(s), func(r rune) bool { return !isTokenRune(r) })
}

// origToken is a run located in original text (SPEC §8.5 step 3): code point offsets of a
// maximal L*/N*/M* run of the original, and the tokens of its normalized form (usually one).
type origToken struct {
	start, end int // code point indexes, end exclusive
	norm       []string
}

// origTokens locates the runs of the original text rs.
func origTokens(rs []rune) []origToken {
	var toks []origToken
	for i := 0; i < len(rs); {
		if !isTokenRune(rs[i]) {
			i++
			continue
		}
		j := i
		for j < len(rs) && isTokenRune(rs[j]) {
			j++
		}
		toks = append(toks, origToken{i, j, Tokens(string(rs[i:j]))})
		i = j
	}
	return toks
}
