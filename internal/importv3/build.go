package importv3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arrowheadmind/memex/internal/slug"
)

var leadingDateSlug = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(-|$)`)

// idFor is slug.ID with one more rule (SPEC §13.3): a slug that begins with a date loses
// that prefix, so an id never carries two dates; the heading's date stays in the title.
func idFor(date, title string) string {
	sl := slug.Slug(title)
	if m := leadingDateSlug.FindString(sl); m != "" {
		sl = sl[len(m):]
	}
	if sl == "" {
		sl = "entry"
	}
	id := date + "-" + sl
	if len(id) <= slug.MaxIDLength {
		return id
	}
	sum := sha256.Sum256([]byte(title))
	return strings.TrimRight(id[:111], "-") + "-" + hex.EncodeToString(sum[:])[:8]
}

// dropDuplicates marks a secondary-SRC unit whose kept-stem id would collide with a primary
// unit of identical body as dropped (SPEC §13.3), before anything depends on the owner set.
func (im *Importer) dropDuplicates() {
	type key struct{ stem, date string }
	primary := map[key]*Unit{}
	for _, u := range im.units {
		if !u.Secondary && u.Stem != "" && slug.ValidID(u.Stem) && u.Stem[:10] == u.Date {
			primary[key{u.Stem, u.Date}] = u
		}
	}
	for _, u := range im.units {
		if !u.Secondary || u.Stem == "" || !slug.ValidID(u.Stem) || u.Stem[:10] != u.Date {
			continue
		}
		if p, ok := primary[key{u.Stem, u.Date}]; ok && strings.Join(p.BodyLines, "\n") == strings.Join(u.BodyLines, "\n") {
			u.Kind = "dropped"
			im.report.Duplicates = append(im.report.Duplicates, dupRec{Source: u.Src.Basename + ":" + u.File, DuplicateOf: p.Stem})
		}
	}
}

// emptyRepoWords are INDEX/BRAIN_REPO values that name no repository (SPEC §13.3). A value
// equal to the basename of a SRC's parent directory (the workspace) means empty as well.
var emptyRepoWords = map[string]bool{"unknown": true, "root": true, "all": true, "global": true}

func (im *Importer) emptyRepo(v string) bool {
	if emptyRepoWords[v] {
		return true
	}
	for _, s := range im.srcs {
		if tagify(filepath.Base(filepath.Dir(s.Abs))) == v {
			return true
		}
	}
	return false
}

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

// datingWorkers is how many git processes date line units at once. The result does not
// depend on it; it only bounds the wall-clock cost of `git log -L` on a large brain.
const datingWorkers = 8

// resolveDates fills the date of every unit (SPEC §13.3 common rules) and the verified date
// of reference docs. Line units are dated concurrently.
func (im *Importer) resolveDates() error {
	var lineUnits []*Unit
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
			lineUnits = append(lineUnits, u)
		default:
			d, err := u.Src.FirstDay(u.File)
			if err != nil {
				return err
			}
			u.Date = d
		}
	}
	started := time.Now()
	errs := make([]error, len(lineUnits))
	var wg sync.WaitGroup
	sem := make(chan struct{}, datingWorkers)
	for i, u := range lineUnits {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, u *Unit) {
			defer wg.Done()
			defer func() { <-sem }()
			errs[i] = im.dateLineUnit(u)
		}(i, u)
	}
	wg.Wait()
	im.datingCost = time.Since(started)
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// dateLineUnit dates one line unit: the oldest commit of `git log -L` over its lines or of
// `git log -S` for its first line, whichever is earlier (SPEC §13.3).
func (im *Importer) dateLineUnit(u *Unit) error {
	lo, hi := u.Lines[0], u.Lines[0]
	for _, n := range u.Lines {
		if n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	first := ""
	for _, l := range u.BodyLines {
		if strings.TrimSpace(l) != "" {
			first = l
			break
		}
	}
	if len(u.TitleLines) > 0 {
		first = ""
		if f := im.fileOf(u); f != nil {
			first = f.Lines[u.TitleLines[0]-1]
		}
	}
	d, err := u.Src.LineRangeDay(u.File, lo, hi, first)
	if err != nil {
		return err
	}
	u.Date = d
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
		base := idFor(u.Date, u.Title)
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
			base = idFor(u.Date, u.Title)
		}
		if _, ok := im.ids[base]; ok {
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
