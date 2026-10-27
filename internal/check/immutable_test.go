package check

import (
	"testing"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

func TestLockedJournalBody(t *testing.T) {
	body := "The checked journal body.\n"
	fields := entry.Fields{"body_sha256": entry.BodySHA256(body)}
	p := &parsed{File: store.File{Type: spec.LookupType("journal")}, body: body, fields: fields}
	head := []byte(entry.Render(entry.Fields{"title": "Committed", "body_sha256": entry.BodySHA256(body)}, body))

	if !lockedJournalBody(p, head) {
		t.Fatal("matching HEAD body and body_sha256 should lock the checked view")
	}
	frontmatterChanged := []byte(entry.Render(entry.Fields{"title": "Changed", "keywords": []string{"new"}}, body))
	if !lockedJournalBody(p, frontmatterChanged) {
		t.Fatal("different HEAD frontmatter should not unlock an identical body")
	}

	tests := []struct {
		name string
		edit func(*parsed) []byte
	}{
		{"no HEAD file", func(*parsed) []byte { return nil }},
		{"checked body differs from HEAD", func(p *parsed) []byte {
			p.body = "A different checked body.\n"
			p.fields["body_sha256"] = entry.BodySHA256(p.body)
			return head
		}},
		{"missing hash", func(p *parsed) []byte { delete(p.fields, "body_sha256"); return head }},
		{"malformed hash", func(p *parsed) []byte { p.fields["body_sha256"] = "ABC"; return head }},
		{"mismatched hash", func(p *parsed) []byte { p.fields["body_sha256"] = entry.BodySHA256("other"); return head }},
		{"not a journal", func(p *parsed) []byte { p.Type = spec.LookupType("rule"); return head }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := *p
			q.fields = entry.Fields{"body_sha256": entry.BodySHA256(body)}
			q.body = body
			if lockedJournalBody(&q, tt.edit(&q)) {
				t.Fatal("body unexpectedly locked")
			}
		})
	}
}
