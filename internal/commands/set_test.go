package commands

import (
	"fmt"
	"testing"
)

func TestSplitWords(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{`id title=x`, `[id title=x]`},
		{"  id\t'title=A b'  tags+=c ", `[id title=A b tags+=c]`},
		{`id "title=A \"b\" \\ c"`, `[id title=A "b" \ c]`},
		{`id title=A\ b`, `[id title=A b]`},
		{`id title=a#b`, `[id title=a#b]`},
		{`id 'title=`, `unterminated single quote`},
		{`id "title=`, `unterminated double quote`},
	} {
		words, problem := splitWords(c.line)
		got := problem
		if problem == "" {
			got = fmt.Sprint(words)
		}
		if got != c.want {
			t.Errorf("splitWords(%q) = %s, want %s", c.line, got, c.want)
		}
	}
}
