package search

import (
	"fmt"
	"strings"

	"github.com/arrowheadmind/memex/internal/jsonx"
)

// matchedItems is the matched items of h as results report them.
func (r *Result) matchedItems(h Hit) []string {
	out := make([]string, len(h.Matched))
	for i, q := range h.Matched {
		out[i] = r.Items[q].String()
	}
	return out
}

// Block is the text block of the i-th shown result (SPEC §8.5): the result line, the title
// line and, when a body line matches, the snippet line, each ending in LF.
func (r *Result) Block(i int) string {
	h := r.Shown[i]
	e := h.Entry
	var b strings.Builder
	fmt.Fprintf(&b, "%d. [%s, %s] %s  matched %d/%d: %s\n", i+1, e.Type.Name, e.Status(), e.Path,
		len(h.Matched), len(r.Items), strings.Join(r.matchedItems(h), ", "))
	b.WriteString("   " + e.Fields.String("title"))
	if s := e.Fields.String("summary"); s != "" {
		b.WriteString(" — " + s)
	}
	b.WriteString("\n")
	if sn := MakeSnippet(e.Body, r.Items); sn != nil {
		b.WriteString("   " + sn.String(true) + "\n")
	}
	return b.String()
}

// Weak reports whether the top shown result covers fewer than half of at least two query
// items (SPEC §8.5): 2k < n with n >= 2. It is false when nothing is shown.
func (r *Result) Weak() bool {
	n := len(r.Items)
	return len(r.Shown) > 0 && n >= 2 && 2*len(r.Shown[0].Matched) < n
}

// Trailer is the weak-results line and the count lines, then, when at least one block is
// printed, the Store root line (SPEC §8.5). The weak line needs a printed block too.
func (r *Result) Trailer(root string, printed int) string {
	var b strings.Builder
	if printed > 0 && r.Weak() {
		fmt.Fprintf(&b, "weak results: the best match covers %d of %d query items; try other words\n",
			len(r.Shown[0].Matched), len(r.Items))
	}
	if r.More > 0 {
		fmt.Fprintf(&b, "%d more visible matches not shown (raise --limit)\n", r.More)
	}
	if r.Hidden > 0 {
		fmt.Fprintf(&b, "%d matches hidden by status (use --all)\n", r.Hidden)
	}
	if r.Excluded > 0 {
		fmt.Fprintf(&b, "%d matches excluded by filters (drop --type/--tag/--repo/--host)\n", r.Excluded)
	}
	if printed > 0 {
		fmt.Fprintf(&b, "Store root: %s\n", root)
	}
	return b.String()
}

// NoMatch is the line printed when no document matched at all (SPEC §8.5).
func (r *Result) NoMatch() string {
	return "No entries matched any term of: " + r.Text + "\n"
}

// TextOutput is the whole text output of SPEC §8.5 for a store at root.
func (r *Result) TextOutput(root string) string {
	if !r.Matched() {
		return r.NoMatch()
	}
	var b strings.Builder
	for i := range r.Shown {
		b.WriteString(r.Block(i))
	}
	b.WriteString(r.Trailer(root, len(r.Shown)))
	return b.String()
}

// JSON is the --json output of SPEC §8.5.
func (r *Result) JSON() []byte {
	items := make([]string, len(r.Items))
	for i, it := range r.Items {
		items[i] = it.String()
	}
	results := make(jsonx.Array, len(r.Shown))
	for i, h := range r.Shown {
		e := h.Entry
		summary, snippet := jsonx.Null, jsonx.Null
		if s := e.Fields.String("summary"); s != "" {
			summary = jsonx.String(s)
		}
		if sn := MakeSnippet(e.Body, r.Items); sn != nil {
			snippet = jsonx.String(sn.String(false))
		}
		results[i] = jsonx.Object{
			{Key: "rank", Value: jsonx.Int(i + 1)},
			{Key: "type", Value: jsonx.String(e.Type.Name)},
			{Key: "status", Value: jsonx.String(e.Status())},
			{Key: "id", Value: jsonx.String(e.ID)},
			{Key: "title", Value: jsonx.String(e.Fields.String("title"))},
			{Key: "summary", Value: summary},
			{Key: "score", Value: jsonx.Score(h.Score)},
			{Key: "path", Value: jsonx.String(e.Path)},
			{Key: "matched", Value: jsonx.Strings(r.matchedItems(h))},
			{Key: "snippet", Value: snippet},
		}
	}
	return jsonx.Encode(jsonx.Object{
		{Key: "query", Value: jsonx.String(r.Text)},
		{Key: "items", Value: jsonx.Strings(items)},
		{Key: "results", Value: results},
		{Key: "more", Value: jsonx.Int(r.More)},
		{Key: "hidden", Value: jsonx.Int(r.Hidden)},
		{Key: "excluded", Value: jsonx.Int(r.Excluded)},
		{Key: "weak", Value: jsonx.Bool(r.Weak())},
	})
}
