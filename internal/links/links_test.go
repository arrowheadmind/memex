package links

import (
	"fmt"
	"testing"
)

func TestExtract(t *testing.T) {
	body := "See [[2026-01-15-a]] and [[old name|text]].\n" +
		"Shell: [[ -f x ]] stays text.\n" +
		"`[[2026-01-15-code]]` and ``[[2026-01-15-x` ``\n" +
		"```\n[[2026-01-15-fenced]] ![i](files/x/y.png)\n```\n" +
		"Unclosed `tick [[2026-01-15-b]] and [img](files/2026-01-15-a/p.png) [l](<files/a b>)\n" +
		"Not a files link: [x](https://x/files/y) [[]] [[a]b]]\n"
	refs, files := Extract(body)
	got := fmt.Sprint(refs, files)
	want := "[{2026-01-15-a 1} {old name 1} { -f x  2} {2026-01-15-b 7}] [{files/2026-01-15-a/p.png 7} {files/a b 7}]"
	if got != want {
		t.Errorf("Extract:\n got %s\nwant %s", got, want)
	}
}

func TestStripCodeSpans(t *testing.T) {
	for in, want := range map[string]string{
		"a `b` c":     "a     c",
		"a ``b`c`` d": "a         d",
		"a `` b ` c":  "a `` b ` c",
		"```x``` y":   "        y",
		"no ticks":    "no ticks",
		"`a` `b":      "    `b",
	} {
		if got := StripCodeSpans(in); got != want {
			t.Errorf("StripCodeSpans(%q) = %q, want %q", in, got, want)
		}
	}
}
