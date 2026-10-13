package importv3

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arrowheadmind/memex/internal/slug"
)

// emptyRepos are INDEX/BRAIN_REPO values that named the workspace, not a repository.
var emptyRepos = map[string]bool{"programming": true, "unknown": true, "root": true, "all": true, "global": true}

// journalStatus maps a source status value to a journal status and a was-* tag.
func journalStatus(v string) (status, was string) {
	switch v {
	case "", "raw", "in-progress":
		if v == "in-progress" {
			return "raw", "was-in-progress"
		}
		return "raw", ""
	case "promoted":
		return "promoted", ""
	case "kept":
		return "kept", ""
	case "superseded":
		return "superseded", ""
	default:
		return "kept", "was-" + tagify(v)
	}
}

// planStatus maps a plan's status value (SPEC §13.3).
func planStatus(v string) (status, was string) {
	switch v {
	case "done", "complete", "completed", "executed", "shipped", "landed":
		if v != "done" {
			return "done", "was-" + v
		}
		return "done", ""
	case "proposal", "proposed", "planned", "ready", "draft", "not":
		if v != "draft" {
			return "draft", "was-" + v
		}
		return "draft", ""
	case "", "active":
		return "active", ""
	default:
		return "active", "was-" + tagify(v)
	}
}

// resolveDates fills the date of every unit (SPEC §13.3 common rules) and the verified date
// of reference docs.
func (im *Importer) resolveDates() error {
	for _, u := range im.units {
		if u.Kind == "doc" && u.Verified == "" {
			last, err := u.Src.LastDay(u.File)
			if err != nil {
				return err
			}
			u.Verified = last
		}
		if u.Date != "" {
			continue
		}
		switch u.Kind {
		case "leak":
			u.Date = im.today
		case "km-row", "km-section", "c-row", "c-section", "always", "task":
			days, err := u.Src.BlameDays(u.File)
			if err != nil {
				return err
			}
			min := ""
			for _, n := range u.Lines {
				if n < len(days) && days[n] != "" && (min == "" || days[n] < min) {
					min = days[n]
				}
			}
			if min == "" {
				d, err := u.Src.FirstDay(u.File)
				if err != nil {
					return err
				}
				min = d
			}
			u.Date = min
		default:
			d, err := u.Src.FirstDay(u.File)
			if err != nil {
				return err
			}
			u.Date = d
		}
	}
	return nil
}

// resolveStatuses applies the status mapping and the marker tags.
func (im *Importer) resolveStatuses() {
	for _, u := range im.units {
		switch u.Kind {
		case "investigation":
			if u.StatusValue == "" && u.IndexStatus != "" {
				u.StatusValue = u.IndexStatus
			}
			st, was := journalStatus(u.StatusValue)
			u.Status = st
			if was != "" {
				u.Tags = append(u.Tags, was)
			}
		case "decision":
			u.Status = "accepted"
			if strings.Contains(u.StatusValue, "supersed") {
				u.Status = "superseded" // confirmed by resolveSuperseded, else accepted + tag
			}
		case "runbook", "doc", "remainder", "attachments":
			u.Status = "active"
		case "plan":
			st, was := planStatus(u.StatusValue)
			u.Status = st
			if was != "" {
				u.Tags = append(u.Tags, was)
			}
		case "prompt", "report":
			u.Status = "done"
		case "leak":
			u.Status = "raw"
		}
		if u.Type == "rule" {
			text := u.Title + "\n" + firstParagraph(strings.Join(u.BodyLines, "\n"))
			u.Tags = append(u.Tags, markerTags(text)...)
		}
	}
}

// assignIDs computes ids in unit order, keeping valid filename stems when unique, with -2,
// -3 suffixes for collisions, and the -<repo> rule for secondary sources (SPEC §13.3).
func (im *Importer) assignIDs() {
	im.ids = map[string]*Unit{}
	bodyOf := func(u *Unit) string { return strings.Join(u.BodyLines, "\n") }
	// stems: a stem is kept only by the first unit, in order, that has it and whose stem is a
	// valid id whose date part equals the unit's date
	stemTaken := map[string]bool{}
	var kept []*Unit
	for _, u := range im.units {
		if u.Secondary || u.Stem == "" || !slug.ValidID(u.Stem) || stemTaken[u.Stem] {
			continue
		}
		if u.Stem[:10] != u.Date {
			continue
		}
		stemTaken[u.Stem] = true
		kept = append(kept, u)
	}
	for _, u := range kept {
		u.ID = u.Stem
		im.ids[u.ID] = u
	}
	for _, u := range im.units {
		if u.ID != "" || u.Secondary || u.Kind == "dropped" {
			continue
		}
		base := slug.ID(u.Date, u.Title)
		u.ID = im.uniqueID(base, "")
		im.ids[u.ID] = u
		if u.Stem != "" && u.Stem != u.ID && slug.ValidID(u.Stem) {
			u.Aliases = append(u.Aliases, u.Stem)
		}
	}
	for _, u := range im.units {
		if !u.Secondary || u.Kind == "dropped" {
			continue
		}
		base := u.Stem
		if base == "" || !slug.ValidID(base) || base[:10] != u.Date {
			base = slug.ID(u.Date, u.Title)
		}
		if other, ok := im.ids[base]; ok {
			if bodyOf(other) == bodyOf(u) {
				u.Kind = "dropped"
				u.ID = ""
				im.report.Duplicates = append(im.report.Duplicates, dupRec{Source: u.Src.Basename + ":" + u.File, DuplicateOf: other.ID})
				continue
			}
			base = im.uniqueID(base, "-"+tagify(u.Src.Basename))
		} else {
			base = im.uniqueID(base, "")
		}
		u.ID = base
		im.ids[u.ID] = u
	}
}

// uniqueID returns base+suffix, then -2, -3, … until unused, within the length cap.
func (im *Importer) uniqueID(base, suffix string) string {
	try := func(s string) string {
		id := base
		if len(id)+len(s) > slug.MaxIDLength {
			id = strings.TrimRight(id[:slug.MaxIDLength-len(s)], "-")
		}
		return id + s
	}
	cand := try(suffix)
	if _, used := im.ids[cand]; !used {
		return cand
	}
	for n := 2; ; n++ {
		cand = try(fmt.Sprintf("%s-%d", suffix, n))
		if _, used := im.ids[cand]; !used {
			return cand
		}
	}
}

// resolveAliases builds each unit's aliases (SPEC §13.3) and drops any that repeat or equal
// an id.
func (im *Importer) resolveAliases() {
	seen := map[string]*Unit{}
	for _, u := range im.units {
		if u.Kind == "dropped" {
			continue
		}
		var al []string
		prefix := ""
		if u.Secondary {
			prefix = u.Src.Basename + "/"
		}
		switch u.Kind {
		case "investigation", "decision", "runbook", "plan", "prompt", "report", "doc":
			rel := u.File
			al = append(al, prefix+rel)
			for _, p := range []string{".brain/", "docs/"} {
				if strings.HasPrefix(rel, p) {
					al = append(al, prefix+strings.TrimPrefix(rel, p))
				}
			}
			if u.Stem != "" && u.Stem != u.ID && !slug.ValidID(u.Stem) {
				al = append(al, prefix+u.Stem)
			}
		case "km-row", "km-section", "c-row", "c-section", "task":
			al = append(al, prefix+u.File+"#"+slug.Slug(u.Title))
		case "always":
			for _, a := range u.Aliases {
				al = append(al, prefix+a)
			}
			u.Aliases = nil
		case "remainder", "attachments":
			al = append(al, prefix+u.File)
		case "leak":
			al = append(al, u.File)
		}
		al = append(u.Aliases, al...)
		var out []string
		for _, a := range al {
			if _, isID := im.ids[a]; isID {
				im.report.AliasDropped = append(im.report.AliasDropped, fmt.Sprintf("%s: %s is an id", u.ID, a))
				continue
			}
			cand := a
			for n := 2; ; n++ {
				if prev, ok := seen[cand]; !ok || prev == u {
					break
				}
				cand = fmt.Sprintf("%s-%d", a, n)
			}
			seen[cand] = u
			out = append(out, cand)
		}
		u.Aliases = dedupe(out)
	}
}

func dedupe(ss []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func sortedTags(ss []string) []string {
	out := dedupe(ss)
	sort.Strings(out)
	return out
}
