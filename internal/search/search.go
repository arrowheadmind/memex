package search

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// BM25 parameters (SPEC §8.3).
const (
	k1 = 1.2
	b  = 0.75
)

// fieldWeights are w_c in the order of SPEC §8.1's table: title, keywords, tags, aliases,
// body.
var fieldWeights = [5]float64{10, 8, 5, 3, 1}

// typeWeights are the type weights of SPEC §8.3.
var typeWeights = map[string]float64{
	"rule": 1.5, "runbook": 1.3, "reference": 1.3, "decision": 1.2,
	"task": 1.0, "plan": 1.0, "journal": 0.8,
}

type document struct {
	entry  *store.Entry
	fields [5][]string // tokens of each §8.1 field, in table order
	length int         // |D|
}

// Corpus is every entry of the store as a search document (SPEC §8.1).
type Corpus struct {
	docs  []*document
	avgdl float64
}

// NewCorpus builds the corpus of entries; N and avgdl cover all of them, whatever their
// status.
func NewCorpus(entries []*store.Entry) *Corpus {
	c := &Corpus{}
	total := 0
	for _, e := range entries {
		f := e.Fields
		keywords := strings.Join(f.Strings("keywords"), " ")
		if s := f.String("summary"); s != "" {
			keywords += " " + s
		}
		d := &document{entry: e, fields: [5][]string{
			Tokens(f.String("title")),
			Tokens(keywords),
			Tokens(strings.Join(f.Strings("tags"), " ")),
			Tokens(strings.Join(f.Strings("aliases"), " ")),
			Tokens(e.Body),
		}}
		for _, toks := range d.fields {
			d.length += len(toks)
		}
		total += d.length
		c.docs = append(c.docs, d)
	}
	if len(c.docs) > 0 {
		c.avgdl = float64(total) / float64(len(c.docs))
	}
	return c
}

// Hit is one matching document.
type Hit struct {
	Entry   *store.Entry
	Score   float64 // the rounded score (SPEC §8.3)
	Matched []int   // indexes of the matched items, in query order
}

// Round rounds a score to 9 significant decimal digits, half to even on the exact binary
// value, and parses it back (SPEC §8.3).
func Round(s float64) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(s, 'g', 9, 64), 64)
	if err != nil {
		panic(err)
	}
	return r
}

// Search scores every document against items and returns the matching ones ranked
// (SPEC §8.3): descending rounded score, then type in §5.2 order, then ascending id.
//
// The arithmetic follows SPEC §8.3 operation by operation, in its order. Every intermediate
// product is wrapped in float64(), which the Go specification defines as rounding, so that
// the compiler cannot fuse a multiply and an add into one FMA instruction.
func (c *Corpus) Search(items []Item) []Hit {
	N := float64(len(c.docs))
	// hits[d][q] is f(q,D); counts[q] is n(q).
	f := make([][]float64, len(c.docs))
	counts := make([]int, len(items))
	for i, d := range c.docs {
		f[i] = make([]float64, len(items))
		for q, it := range items {
			sum := 0.0
			for fc, toks := range d.fields {
				sum = sum + float64(fieldWeights[fc]*float64(it.hits(toks)))
			}
			f[i][q] = sum
			if sum > 0 {
				counts[q]++
			}
		}
	}
	idf := make([]float64, len(items))
	for q := range items {
		n := float64(counts[q])
		idf[q] = math.Log(1 + (N-n+0.5)/(n+0.5))
	}
	var out []Hit
	for i, d := range c.docs {
		var matched []int
		for q := range items {
			if f[i][q] > 0 {
				matched = append(matched, q)
			}
		}
		if matched == nil {
			continue
		}
		dl := float64(d.length)
		norm := float64(1-b) + float64(b*dl)/c.avgdl
		bm25 := 0.0
		for q := range items {
			fq := f[i][q]
			num := float64(float64(idf[q]*fq) * (k1 + 1))
			bm25 = bm25 + num/(fq+float64(k1*norm))
		}
		score := float64(bm25 * typeWeights[d.entry.Type.Name])
		out = append(out, Hit{Entry: d.entry, Score: Round(score), Matched: matched})
	}
	slices.SortStableFunc(out, func(x, y Hit) int {
		switch {
		case x.Score > y.Score:
			return -1
		case x.Score < y.Score:
			return 1
		}
		if c := typeIndex(x.Entry.Type) - typeIndex(y.Entry.Type); c != 0 {
			return c
		}
		return strings.Compare(x.Entry.ID, y.Entry.ID)
	})
	return out
}

func typeIndex(t *spec.Type) int {
	for i := range spec.Types {
		if spec.Types[i].Name == t.Name {
			return i
		}
	}
	panic("unknown type " + t.Name)
}
