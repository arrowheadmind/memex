package importv3

import (
	"regexp"
	"sort"
	"strings"

	"github.com/arrowheadmind/memex/internal/slug"
)

// refDirs are the source directories a path reference may name (SPEC §13.5), longest first.
var refDirs = []string{"plans/prompts", "plans/reports", "docs/runbooks", "investigations", "decisions", "plans", "runbooks", "docs"}

var refRe = regexp.MustCompile(`(\.brain/)?(plans/prompts|plans/reports|docs/runbooks|investigations|decisions|plans|runbooks|docs)/([A-Za-z0-9._-]+\.md)(:[0-9]+|#L[0-9]+)?`)

var wikiRe = regexp.MustCompile(`\[\[([^\]|\n]+)(\|[^\]\n]*)?\]\]`)

// keysOf returns the resolution keys of a file unit's path: "investigations/x.md",
// "docs/runbooks/x.md", and for runbooks also the bare "runbooks/x.md".
func keysOf(p string) []string {
	rel := strings.TrimPrefix(p, ".brain/")
	keys := []string{rel}
	if strings.HasPrefix(rel, "docs/runbooks/") {
		keys = append(keys, "runbooks/"+strings.TrimPrefix(rel, "docs/runbooks/"))
	}
	return keys
}

// buildResolver indexes every file unit by source and key.
func (im *Importer) buildResolver() {
	im.byKey = map[*Source]map[string][]*Unit{}
	for _, u := range im.units {
		if u.Kind == "dropped" || u.Stem == "" {
			continue
		}
		m := im.byKey[u.Src]
		if m == nil {
			m = map[string][]*Unit{}
			im.byKey[u.Src] = m
		}
		for _, k := range keysOf(u.File) {
			m[k] = append(m[k], u)
		}
	}
}

// unitAt returns the unit for an exact source path, if any.
func (im *Importer) unitAt(src *Source, p string) *Unit {
	for _, u := range im.units {
		if u.Src == src && u.File == p && u.Stem != "" && u.Kind != "dropped" {
			return u
		}
	}
	return nil
}

// resolveRef resolves a reference (after prefix stripping) to exactly one unit: first in the
// reference's own source, then in the primary.
func (im *Importer) resolveRef(src *Source, ref string) *Unit {
	ref = strings.TrimPrefix(ref, "./")
	for _, p := range im.allPrefixes() {
		if strings.HasPrefix(ref, p) {
			ref = strings.TrimPrefix(ref, p)
			break
		}
	}
	key := strings.TrimPrefix(ref, ".brain/")
	if m := refRe.FindStringSubmatch(key); m != nil && m[0] == key {
		key = m[2] + "/" + m[3]
	}
	for _, s := range []*Source{src, im.primary} {
		if us := im.byKey[s][key]; len(us) == 1 {
			return us[0]
		} else if len(us) > 1 {
			return nil
		}
	}
	return nil
}

// codeSpans returns the byte ranges of the inline code spans of a line, with CommonMark's
// rule: a run of N backticks is closed by the next run of exactly N (SPEC §6).
func codeSpans(line string) [][2]int {
	var spans [][2]int
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(line) && line[j] == '`' {
			j++
		}
		n := j - i
		k := j
		closed := false
		for k < len(line) {
			if line[k] != '`' {
				k++
				continue
			}
			m := k
			for m < len(line) && line[m] == '`' {
				m++
			}
			if m-k == n {
				spans = append(spans, [2]int{i, m})
				i = m
				closed = true
				break
			}
			k = m
		}
		if !closed {
			i = j
		}
	}
	return spans
}

func inSpans(spans [][2]int, start, end int) bool {
	for _, sp := range spans {
		if start < sp[1] && end > sp[0] {
			return true
		}
	}
	return false
}

// allPrefixes returns every SRC's prefixes (SPEC §13.5), longest first.
func (im *Importer) allPrefixes() []string {
	if im.prefixes != nil {
		return im.prefixes
	}
	set := map[string]bool{}
	for _, s := range im.srcs {
		for _, p := range s.Prefixes(im.home) {
			set[p] = true
		}
	}
	for p := range set {
		im.prefixes = append(im.prefixes, p)
	}
	sort.Slice(im.prefixes, func(i, j int) bool {
		if len(im.prefixes[i]) != len(im.prefixes[j]) {
			return len(im.prefixes[i]) > len(im.prefixes[j])
		}
		return im.prefixes[i] < im.prefixes[j]
	})
	return im.prefixes
}

// boundaryByte reports whether c may precede a path reference: a reference glued to a path
// character on its left (as in `../.brain/x.md`) is part of a longer path and stays.
func boundaryByte(c byte) bool {
	return !(c == '.' || c == '/' || c == '_' || c == '-' || c == '~' ||
		('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9'))
}

var suffixRe = regexp.MustCompile(`(:[0-9]+|#L[0-9]+)$`)

// rewriteLinks replaces resolvable path references with [[id]] in a body (SPEC §13.5). A
// reference wrapped in single backticks loses them, because a link inside a code span is not
// a link (§6); a reference inside any other code span, or in fenced code, stays as it is.
// Unresolvable wikilinks that look like ids become inline code.
func (im *Importer) rewriteLinks(u *Unit, body string) string {
	lines := strings.Split(body, "\n")
	code := fenced(lines)
	full, wrapped := im.refPatterns()
	rewrite := func(m string) (string, bool) {
		sm := full.FindStringSubmatch(m)
		suffix := sm[len(sm)-1]
		target := strings.TrimSuffix(m, suffix)
		if t := im.resolveRef(u.Src, target); t != nil {
			return "[[" + t.ID + "]]" + suffix, true
		}
		im.unresolved[target]++
		return m, false
	}
	for i, l := range lines {
		if code[i] {
			continue
		}
		// pass 1: `ref` -> [[id]]
		{
			var b strings.Builder
			last := 0
			for _, loc := range wrapped.FindAllStringIndex(l, -1) {
				if loc[0] > 0 && !boundaryByte(l[loc[0]-1]) {
					continue
				}
				m := l[loc[0]:loc[1]]
				inner := m[1 : len(m)-1]
				b.WriteString(l[last:loc[0]])
				if to, ok := rewrite(inner); ok {
					im.report.Links.Rewritten = append(im.report.Links.Rewritten, rewriteRec{Entry: u.ID, From: m, To: to})
					b.WriteString(to)
				} else {
					b.WriteString(m)
				}
				last = loc[1]
			}
			b.WriteString(l[last:])
			l = b.String()
		}
		// pass 2: bare references outside code spans
		spans := codeSpans(l)
		var b strings.Builder
		last := 0
		for _, loc := range full.FindAllStringIndex(l, -1) {
			if inSpans(spans, loc[0], loc[1]) || (loc[0] > 0 && !boundaryByte(l[loc[0]-1])) {
				continue
			}
			m := l[loc[0]:loc[1]]
			to, ok := rewrite(m)
			b.WriteString(l[last:loc[0]])
			if ok {
				im.report.Links.Rewritten = append(im.report.Links.Rewritten, rewriteRec{Entry: u.ID, From: m, To: to})
			}
			b.WriteString(to)
			last = loc[1]
		}
		b.WriteString(l[last:])
		l = b.String()
		l = wikiRe.ReplaceAllStringFunc(l, func(m string) string {
			sm := wikiRe.FindStringSubmatch(m)
			target := sm[1]
			if _, ok := im.ids[target]; ok {
				return m
			}
			if al := im.aliasOwner[target]; al != nil {
				return m // resolves through the alias (W-ALIAS-LINK)
			}
			if slug.ValidID(target) {
				im.report.Links.WikiQuoted = append(im.report.Links.WikiQuoted, rewriteRec{Entry: u.ID, From: m, To: "`" + m + "`"})
				return "`" + m + "`"
			}
			return m // plain text per §6
		})
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// refPatterns compiles, once, the reference pattern with every SRC's prefixes and its
// single-backtick-wrapped form.
func (im *Importer) refPatterns() (*regexp.Regexp, *regexp.Regexp) {
	if im.fullRe == nil {
		var alt []string
		for _, p := range im.allPrefixes() {
			alt = append(alt, regexp.QuoteMeta(p))
		}
		im.fullRe = regexp.MustCompile(`(?:` + strings.Join(alt, "|") + `)?` + refRe.String())
		im.wrappedRe = regexp.MustCompile("`" + im.fullRe.String() + "`")
	}
	return im.fullRe, im.wrappedRe
}

// resolveSuperseded finds the "superseded by" target of units whose status says so.
func (im *Importer) resolveSuperseded() {
	for _, u := range im.units {
		if u.Kind == "dropped" {
			continue
		}
		wants := u.Status == "superseded"
		if !wants {
			continue
		}
		var target *Unit
		for _, h := range u.SupHints {
			if !strings.Contains(strings.ToLower(strings.Join(u.SupHints, "\n")), "supersed") {
				break
			}
			for _, m := range refRe.FindAllString(h, -1) {
				m = strings.TrimSuffix(m, suffixRe.FindString(m))
				if t := im.resolveRef(u.Src, m); t != nil && t != u {
					target = t
					break
				}
			}
			if target != nil {
				break
			}
		}
		if target != nil {
			u.SupersededBy = target.ID
			continue
		}
		u.Tags = append(u.Tags, "was-superseded")
		switch u.Type {
		case "journal":
			u.Status = "kept"
		case "decision":
			u.Status = "accepted"
		default:
			u.Status = "active"
		}
	}
}

// unresolvedList returns the unresolved reference counts, sorted by target.
func (im *Importer) unresolvedList() []countRec {
	var out []countRec
	for t, n := range im.unresolved {
		out = append(out, countRec{Target: t, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}
