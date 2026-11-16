// Package links extracts the links of SPEC §6 from an entry body: [[TARGET]] and
// [[TARGET|TEXT]] links, and Markdown links into files/. Links inside fenced code blocks and
// inline code spans are ignored.
package links

import (
	"regexp"
	"strings"

	"github.com/arrowheadmind/memex/internal/entry"
)

// Ref is one [[TARGET]] candidate. Whether it is a link depends on the store: TARGET must
// match the id pattern or equal an alias (SPEC §6).
type Ref struct {
	Target string
	Line   int // 1-based line of the body
}

// FileRef is one Markdown link whose destination starts with files/.
type FileRef struct {
	Path string
	Line int
}

var (
	wikiLink = regexp.MustCompile(`\[\[([^\]|\n]+)(?:\|[^\]\n]*)?\]\]`)
	fileLink = regexp.MustCompile(`\]\((?:<(files/[^>]*)>|(files/[^)\s]*))`)
)

// Extract returns the [[...]] candidates and the files/ links of body, in order.
func Extract(body string) ([]Ref, []FileRef) {
	var refs []Ref
	var files []FileRef
	lines := entry.Lines(body)
	fenced := entry.Fenced(lines)
	for i, line := range lines {
		if fenced[i] {
			continue
		}
		line = StripCodeSpans(line)
		for _, m := range wikiLink.FindAllStringSubmatch(line, -1) {
			refs = append(refs, Ref{m[1], i + 1})
		}
		for _, m := range fileLink.FindAllStringSubmatch(line, -1) {
			files = append(files, FileRef{m[1] + m[2], i + 1})
		}
	}
	return refs, files
}

// StripCodeSpans replaces every inline code span of line with spaces. A span opens with a run
// of n backticks and closes with the next run of exactly n backticks on the same line; a run
// without a closing run is literal text.
func StripCodeSpans(line string) string {
	if !strings.Contains(line, "`") {
		return line
	}
	b := []byte(line)
	for i := 0; i < len(b); {
		if b[i] != '`' {
			i++
			continue
		}
		n := run(b, i)
		closeAt := -1
		for j := i + n; j < len(b); {
			if b[j] != '`' {
				j++
				continue
			}
			m := run(b, j)
			if m == n {
				closeAt = j
				break
			}
			j += m
		}
		if closeAt < 0 {
			i += n
			continue
		}
		for k := i; k < closeAt+n; k++ {
			b[k] = ' '
		}
		i = closeAt + n
	}
	return string(b)
}

func run(b []byte, i int) int {
	n := 0
	for i+n < len(b) && b[i+n] == '`' {
		n++
	}
	return n
}

// Shaped reports whether a [[TARGET]] that is neither an id nor an alias is shaped like a link
// (SPEC §6, broken links): only ASCII letters, digits, '-', '_', '.' and '/', with at least
// one '-', '_' or '/'. Such a TARGET is W-BROKEN-LINK; any other is plain text.
func Shaped(target string) bool {
	sep := false
	for i := 0; i < len(target); i++ {
		switch b := target[i]; {
		case b == '-' || b == '_' || b == '/':
			sep = true
		case b == '.', b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		default:
			return false
		}
	}
	return sep
}
