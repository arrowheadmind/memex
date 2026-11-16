package check

import (
	"strings"
	"testing"
)

func TestMeant(t *testing.T) {
	long := "2026-01-10-" + strings.Repeat("word-", 25) + "end"
	capped := strings.TrimRight(long[:111], "-") + "-"
	shared := "2026-01-11-" + strings.Repeat("other-", 21) + "end"
	sharedCap := strings.TrimRight(shared[:111], "-") + "-"
	c := &checker{byID: map[string][]*parsed{}, byAlias: map[string][]*parsed{}}
	for _, id := range []string{"2026-01-10-target", capped + "1a2b3c4d", sharedCap + "aaaaaaaa", sharedCap + "bbbbbbbb", sharedCap + "ABCDEF12"} {
		c.byID[id] = []*parsed{{ID: id}}
	}
	c.byAlias["former-target"] = c.byID["2026-01-10-target"]
	c.byAlias["shared-alias"] = []*parsed{{ID: "2026-01-10-a"}, {ID: "2026-01-10-b"}}

	for target, want := range map[string]string{
		"rules/2026-01-10-target":                      "2026-01-10-target",
		"journal/2026-01-10-target":                    "2026-01-10-target",
		"2026-01-10-target.md":                         "2026-01-10-target",
		"tasks/2026-01-10-target.md":                   "2026-01-10-target",
		"reference/former-target":                      "2026-01-10-target",
		"former-target.md":                             "2026-01-10-target",
		long:                                           capped + "1a2b3c4d",
		"plans/" + long + ".md":                        capped + "1a2b3c4d",
		shared:                                         "", // two lowercase-hex matches; the uppercase one is no id the cap makes
		"notes/2026-01-10-target":                      "", // notes is not a type directory
		"rules/rules/2026-01-10-target":                "",
		"rules/shared-alias":                           "",
		"2026-01-10-target.md.md":                      "",
		capped + "1a2b3c4d.txt":                        "",
		"old-brain-slug":                               "",
		capped + "a-different-tail-that-the-cap-drops": capped + "1a2b3c4d",
	} {
		if got := c.meant(target); got != want {
			t.Errorf("meant(%q) = %q, want %q", target, got, want)
		}
	}
}
