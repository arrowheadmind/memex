package search

import (
	"strings"
	"unicode"

	"github.com/arrowheadmind/memex/internal/entry"
)

// SnippetWidth is the window of SPEC §8.5 step 4, in code points, and SnippetLead how far
// before the first matching token it starts.
const (
	SnippetWidth = 160
	SnippetLead  = 40
)

// Snippet is a body line cut to its window, with the matching tokens marked.
type Snippet struct {
	text   []rune
	marks  []origToken // matching tokens inside the window, offsets into text
	prefix bool        // the window starts after the line's start
	suffix bool        // the window ends before the line's end
}

// String renders the snippet; with markers, each matching token is wrapped in **.
func (s *Snippet) String(markers bool) string {
	var b strings.Builder
	if s.prefix {
		b.WriteString("…")
	}
	pos := 0
	if markers {
		for _, m := range s.marks {
			b.WriteString(string(s.text[pos:m.start]) + "**" + string(s.text[m.start:m.end]) + "**")
			pos = m.end
		}
	}
	b.WriteString(string(s.text[pos:]))
	if s.suffix {
		b.WriteString("…")
	}
	return b.String()
}

// lineMatches reports whether the §8.2 tokens of line contain a term match or a phrase.
func lineMatches(line string, items []Item) bool {
	toks := Tokens(line)
	for _, it := range items {
		if it.hits(toks) > 0 {
			return true
		}
	}
	return false
}

// MakeSnippet builds the snippet of SPEC §8.5 from body, or returns nil when no body line
// outside fenced code contains a match.
func MakeSnippet(body string, items []Item) *Snippet {
	lines := entry.Lines(body)
	fenced := entry.Fenced(lines)
	for i, line := range lines {
		if fenced[i] || !lineMatches(line, items) {
			continue
		}
		return window([]rune(strings.TrimFunc(line, unicode.IsSpace)), items)
	}
	return nil
}

// matching returns the runs of the original line rs that match an item (SPEC §8.5 step 3): a
// run matches when a token of its normalized form matches a term or belongs to an occurrence
// of a phrase in the line's sequence of run tokens. They are in line order.
func matching(rs []rune, items []Item) []origToken {
	runs := origTokens(rs)
	var stream []string // the tokens of every run, in order
	var owner []int     // the run of each stream token
	for i, r := range runs {
		for _, t := range r.norm {
			stream = append(stream, t)
			owner = append(owner, i)
		}
	}
	hit := make([]bool, len(runs))
	for _, it := range items {
		if !it.Phrase {
			for k, t := range stream {
				if it.matchToken(t) {
					hit[owner[k]] = true
				}
			}
			continue
		}
		n := len(it.Tokens)
		for k := 0; k+n <= len(stream); k++ {
			if phraseAt(stream, k, it.Tokens) {
				for j := range n {
					hit[owner[k+j]] = true
				}
			}
		}
	}
	var out []origToken
	for i, r := range runs {
		if hit[i] {
			out = append(out, r)
		}
	}
	return out
}

// window applies steps 3 to 5 of SPEC §8.5 to the trimmed line rs.
func window(rs []rune, items []Item) *Snippet {
	marks := matching(rs, items)
	start, end := 0, len(rs)
	if len(rs) > SnippetWidth {
		if len(marks) > 0 {
			start = max(marks[0].start-SnippetLead, 0)
		}
		start = min(start, len(rs)-SnippetWidth)
		end = start + SnippetWidth
		// A matching run cut by an edge is dropped from the window, with the whitespace that
		// the drop leaves at that edge.
		droppedStart, droppedEnd := false, false
		for _, m := range marks {
			if m.start < start && m.end > start {
				start, droppedStart = m.end, true
			}
			if m.start < end && m.end > end {
				end, droppedEnd = m.start, true
			}
		}
		end = max(end, start) // only a matching run longer than the window can cross both
		for droppedStart && start < end && unicode.IsSpace(rs[start]) {
			start++
		}
		for droppedEnd && end > start && unicode.IsSpace(rs[end-1]) {
			end--
		}
	}
	s := &Snippet{text: rs[start:end], prefix: start > 0, suffix: end < len(rs)}
	for _, m := range marks {
		if m.start >= start && m.end <= end {
			s.marks = append(s.marks, origToken{m.start - start, m.end - start, m.norm})
		}
	}
	return s
}
