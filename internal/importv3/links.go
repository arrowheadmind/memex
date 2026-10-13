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
	for _, p := range src.Prefixes(im.home) {
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

// rewriteLinks replaces resolvable path references with [[id]] in a body and makes
// unresolvable existing wikilinks inline code (SPEC §13.5).
func (im *Importer) rewriteLinks(u *Unit, body string) string {
	lines := strings.Split(body, "\n")
	code := fenced(lines)
	prefixes := u.Src.Prefixes(im.home)
	var prefixAlt []string
	for _, p := range prefixes {
		prefixAlt = append(prefixAlt, regexp.QuoteMeta(p))
	}
	full := regexp.MustCompile(`(?:` + strings.Join(prefixAlt, "|") + `)?` + refRe.String())
	for i, l := range lines {
		if code[i] {
			continue
		}
		l = full.ReplaceAllStringFunc(l, func(m string) string {
			sm := full.FindStringSubmatch(m)
			suffix := sm[len(sm)-1]
			target := strings.TrimSuffix(m, suffix)
			if t := im.resolveRef(u.Src, target); t != nil {
				im.report.Links.Rewritten = append(im.report.Links.Rewritten, rewriteRec{Entry: u.ID, From: m, To: "[[" + t.ID + "]]" + suffix})
				return "[[" + t.ID + "]]" + suffix
			}
			im.unresolved[target]++
			return m
		})
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
				m = strings.TrimSuffix(m, regexp.MustCompile(`(:[0-9]+|#L[0-9]+)$`).FindString(m))
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
