package commands

import (
	"reflect"
	"testing"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

func TestJournalAliasLinks(t *testing.T) {
	target := &store.Entry{File: store.File{ID: "2026-01-10-target", Type: spec.LookupType("rule")},
		Fields: entry.Fields{"aliases": []string{"notes/target.md", "shared"}}}
	other := &store.Entry{File: store.File{ID: "2026-01-10-other", Type: spec.LookupType("rule")},
		Fields: entry.Fields{"aliases": []string{"shared"}}}
	p := &prepared{typ: spec.LookupType("journal"), id: "2026-01-15-journal",
		fields: entry.Fields{"aliases": []string{"notes/self.md"}},
		body:   "[[notes/target.md]]\n`[[notes/target.md]]`\n```\n[[notes/target.md]]\n```\n[[shared]]\n[[notes/self.md]]\n[[2026-01-10-target]]\n"}
	want := []aliasLinkRefusal{
		{target: "notes/target.md", line: 1, id: "2026-01-10-target"},
		{target: "notes/self.md", line: 7, id: "2026-01-15-journal"},
	}
	if got := journalAliasLinks(p, []*store.Entry{target, other}); !reflect.DeepEqual(got, want) {
		t.Fatalf("journalAliasLinks() = %#v, want %#v", got, want)
	}
	p.typ = spec.LookupType("rule")
	if got := journalAliasLinks(p, []*store.Entry{target}); got != nil {
		t.Fatalf("rule alias links = %#v, want nil", got)
	}
}
