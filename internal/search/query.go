package search

import (
	"strings"
	"unicode/utf8"
)

// MinPrefix is the normalized length, in code points, from which a term is a prefix term
// (SPEC §8.3).
const MinPrefix = 4

// Item is one query item (SPEC §8.3): a term (one token) or a phrase (one or more tokens).
type Item struct {
	Phrase bool
	Tokens []string // normalized; a term has exactly one
}

// Prefix reports whether the item is a prefix term.
func (it Item) Prefix() bool {
	return !it.Phrase && utf8.RuneCountInString(it.Tokens[0]) >= MinPrefix
}

// String is the item as results report it (SPEC §8.5): a term's normalized token, or a
// phrase's normalized tokens joined by spaces in double quotes.
func (it Item) String() string {
	if it.Phrase {
		return `"` + strings.Join(it.Tokens, " ") + `"`
	}
	return it.Tokens[0]
}

// matchToken reports whether the term it matches the token tok.
func (it Item) matchToken(tok string) bool {
	if it.Prefix() {
		return strings.HasPrefix(tok, it.Tokens[0])
	}
	return tok == it.Tokens[0]
}

// hits counts the matching tokens of a term, or the occurrences of a phrase, in toks. Phrase
// occurrences are counted at every start position, so they may overlap.
func (it Item) hits(toks []string) int {
	n := 0
	if !it.Phrase {
		for _, t := range toks {
			if it.matchToken(t) {
				n++
			}
		}
		return n
	}
	k := len(it.Tokens)
	for i := 0; i+k <= len(toks); i++ {
		if phraseAt(toks, i, it.Tokens) {
			n++
		}
	}
	return n
}

func phraseAt(toks []string, i int, phrase []string) bool {
	for j, p := range phrase {
		if toks[i+j] != p {
			return false
		}
	}
	return true
}

// Query is a parsed query.
type Query struct {
	Text  string // the query as given
	Items []Item // after dropping duplicates, in query order
}

// ParseQuery parses text by SPEC §8.3. Double quotes pair up from the left; the text between
// a pair is a phrase, and an unmatched last quote is a separator. Every token outside phrases
// is a term. A phrase with no tokens is no item. Duplicate items (same kind, same tokens) are
// dropped, keeping the first. A query with no items is the caller's usage error.
func ParseQuery(text string) Query {
	q := Query{Text: text}
	seen := map[string]bool{}
	add := func(it Item) {
		key := "t"
		if it.Phrase {
			key = "p"
		}
		key += "\x00" + strings.Join(it.Tokens, "\x00")
		if !seen[key] {
			seen[key] = true
			q.Items = append(q.Items, it)
		}
	}
	terms := func(s string) {
		for _, t := range Tokens(s) {
			add(Item{Tokens: []string{t}})
		}
	}
	rest := text
	quotes := strings.Count(text, `"`)
	for pairs := quotes / 2; pairs > 0; pairs-- {
		open := strings.IndexByte(rest, '"')
		terms(rest[:open])
		rest = rest[open+1:]
		end := strings.IndexByte(rest, '"')
		if toks := Tokens(rest[:end]); len(toks) > 0 {
			add(Item{Phrase: true, Tokens: toks})
		}
		rest = rest[end+1:]
	}
	terms(rest) // an unmatched quote is not a token rune, so it separates
	return q
}
