package spec

// MaxSuggestDistance is the largest Levenshtein distance at which a name is suggested
// (SPEC §16.2, §17.4).
const MaxSuggestDistance = 3

// Nearest returns the candidate with the smallest Levenshtein distance to name, counted in
// code points, if that distance is at most MaxSuggestDistance. Ties go to the earlier
// candidate, so the caller's order decides (SPEC §16.2).
func Nearest(name string, candidates []string) (string, bool) {
	best, bestDist := "", MaxSuggestDistance+1
	for _, c := range candidates {
		if d := levenshtein(name, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= MaxSuggestDistance
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
