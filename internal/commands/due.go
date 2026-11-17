package commands

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/jsonx"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// DueOptions are the options of `memex due` (SPEC §11.18).
type DueOptions struct {
	Days  int // the horizon is today plus Days
	Hints bool
	JSON  bool
}

// DefaultDueDays is the default --days of due (SPEC §11.18).
const DefaultDueDays = 7

// dueItem is one entry of a due section.
type dueItem struct {
	en      *store.Entry
	due     string // YYYY-MM-DD, or "" when absent
	dueWhen string
}

// hint is a task that names a date only in its text (SPEC §11.18).
type hint struct {
	en   *store.Entry
	date string
}

// Due runs `memex due` (SPEC §11.18). It reads the working tree's entries only.
func Due(e *Env, o DueOptions) int {
	rs, code, ok := e.loadEntries()
	if !ok {
		return code
	}
	today := rs.today
	todayS := today.Format(time.DateOnly)
	horizon := today.AddDate(0, 0, o.Days).Format(time.DateOnly)
	m, _, err := store.ReadManifest(store.WorkTree{Root: rs.root})
	if err != nil {
		panic(err) // loadEntries read it
	}
	var past, upcoming, when, blocked []dueItem
	var hints []hint
	var skipped []store.Skipped
	for _, s := range rs.skipped { // loadEntries reported these on standard error
		if strings.HasPrefix(s.Path, "tasks/") || strings.HasPrefix(s.Path, "plans/") {
			skipped = append(skipped, s)
		}
	}
	later := 0
	for _, en := range rs.entries {
		if en.Type.Name != "task" && en.Type.Name != "plan" {
			continue
		}
		if problem := dueProblem(en, m.Format); problem != "" {
			// Left out, but never silently (SPEC §11.18): a count line, the JSON, standard error.
			fmt.Fprintf(e.Stderr, "memex: skipped %s: %s; run memex check\n", en.Path, problem)
			skipped = append(skipped, store.Skipped{Path: en.Path, Problem: problem})
			continue
		}
		status := en.Status()
		it := dueItem{en: en, dueWhen: en.Fields.String("due_when")}
		if d, ok := en.Fields.Date("due"); ok {
			it.due = d.Format(time.DateOnly)
		}
		parked := en.Type.Name == spec.ParkedType && status == spec.ParkedStatus
		switch {
		case parked:
			blocked = append(blocked, it)
		case !spec.IsOpen(en.Type.Name, status):
		case it.due != "" && it.due < todayS:
			past = append(past, it)
		case it.due != "" && it.due <= horizon:
			upcoming = append(upcoming, it)
		case it.due != "":
			later++
		case it.dueWhen != "":
			when = append(when, it)
		}
		if en.Type.Name == "task" && slices.Contains(en.Type.Visible, status) && it.due == "" && it.dueWhen == "" {
			if d := textDate(en); d != "" {
				hints = append(hints, hint{en, d})
			}
		}
	}
	byDue := func(a, b dueItem) int {
		if c := strings.Compare(a.due, b.due); c != 0 {
			return c
		}
		return strings.Compare(a.en.ID, b.en.ID)
	}
	byID := func(a, b dueItem) int { return strings.Compare(a.en.ID, b.en.ID) }
	slices.SortFunc(past, byDue)
	slices.SortFunc(upcoming, byDue)
	slices.SortFunc(when, byID)
	slices.SortFunc(blocked, byID)
	slices.SortFunc(hints, func(a, b hint) int { return strings.Compare(a.en.ID, b.en.ID) })

	if o.JSON {
		list := func(items []dueItem) jsonx.Array {
			out := make(jsonx.Array, len(items))
			for i, it := range items {
				out[i] = jsonx.Object{
					{Key: "id", Value: jsonx.String(it.en.ID)},
					{Key: "type", Value: jsonx.String(it.en.Type.Name)},
					{Key: "status", Value: jsonx.String(it.en.Status())},
					{Key: "path", Value: jsonx.String(it.en.Path)},
					{Key: "title", Value: jsonx.String(it.en.Fields.String("title"))},
					{Key: "summary", Value: nullable(it.en.Fields.String("summary"))},
					{Key: "due", Value: nullable(it.due)},
					{Key: "due_when", Value: nullable(it.dueWhen)},
				}
			}
			return out
		}
		hs := make(jsonx.Array, len(hints))
		for i, h := range hints {
			hs[i] = jsonx.Object{
				{Key: "id", Value: jsonx.String(h.en.ID)},
				{Key: "status", Value: jsonx.String(h.en.Status())},
				{Key: "path", Value: jsonx.String(h.en.Path)},
				{Key: "title", Value: jsonx.String(h.en.Fields.String("title"))},
				{Key: "date", Value: jsonx.String(h.date)},
			}
		}
		sk := make(jsonx.Array, len(skipped))
		for i, s := range skipped {
			sk[i] = jsonx.Object{
				{Key: "path", Value: jsonx.String(s.Path)},
				{Key: "problem", Value: jsonx.String(s.Problem)},
			}
		}
		e.Stdout.Write(jsonx.Encode(jsonx.Object{
			{Key: "today", Value: jsonx.String(todayS)},
			{Key: "horizon", Value: jsonx.String(horizon)},
			{Key: "past", Value: list(past)},
			{Key: "upcoming", Value: list(upcoming)},
			{Key: "when", Value: list(when)},
			{Key: "blocked", Value: list(blocked)},
			{Key: "later", Value: jsonx.Int(later)},
			{Key: "hints", Value: hs},
			{Key: "skipped", Value: sk},
		}))
		return 0
	}

	var b strings.Builder
	line := func(it dueItem, why bool) {
		d := it.due
		if d == "" {
			d = "-"
		}
		fmt.Fprintf(&b, "%s %s %s %s", d, it.en.Status(), entry.EscapeControl(it.en.Path), entry.EscapeControl(it.en.Fields.String("title")))
		if it.dueWhen != "" {
			fmt.Fprintf(&b, " (when: %s)", entry.EscapeControl(it.dueWhen))
		}
		if s := it.en.Fields.String("summary"); why && s != "" {
			fmt.Fprintf(&b, " (why: %s)", entry.EscapeControl(s))
		}
		b.WriteString("\n")
	}
	section := func(header string, items []dueItem, why bool) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s: %d\n", header, len(items))
		for _, it := range items {
			line(it, why)
		}
	}
	section("past due", past, false)
	section("due by "+horizon, upcoming, false)
	section("when", when, false)
	section("blocked", blocked, true)
	if later > 0 {
		fmt.Fprintf(&b, "due after %s: %d (raise --days)\n", horizon, later)
	}
	if len(hints) > 0 {
		if o.Hints {
			fmt.Fprintf(&b, "dates in text, no due, guessed from the wording: %d; read each entry before setting its date\n", len(hints))
			for _, h := range hints {
				fmt.Fprintf(&b, "%s %s %s %s (if it is a deadline: memex set %s due=%s)\n",
					h.date, h.en.Status(), entry.EscapeControl(h.en.Path), entry.EscapeControl(h.en.Fields.String("title")), h.en.ID, h.date)
			}
		} else {
			fmt.Fprintf(&b, "dates in text, no due, guessed from the wording: %d (memex due --hints lists them)\n", len(hints))
		}
	}
	if b.Len() == 0 {
		b.WriteString("nothing due\n")
	}
	if len(skipped) > 0 {
		fmt.Fprintf(&b, "skipped, not valid: %d (run memex check)\n", len(skipped))
	}
	fmt.Fprint(e.Stdout, b.String())
	return 0
}

// dueProblem returns why memex due cannot classify or print a task or plan, or "": a field
// it reads (status, title, summary, due, due_when) that is missing where required or not
// valid, due or due_when in a store whose format does not allow them, or a control
// character in a printed field where the format refuses it (SPEC §11.18, §5.3).
func dueProblem(en *store.Entry, format int64) string {
	st, ok := en.Fields["status"].(string)
	switch {
	case !en.Fields.Has("status"):
		return "missing required field \"status\""
	case !ok || !slices.Contains(en.Type.Statuses, st):
		return "status is not valid"
	case !en.Fields.Has("title"):
		return "missing required field \"title\""
	}
	for _, f := range []string{"title", "summary", "due", "due_when"} {
		if _, bad := en.Fields[f].(entry.Invalid); bad {
			return f + " is not valid"
		}
	}
	for _, f := range spec.DueFields {
		if en.Fields.Has(f) && format < spec.DueFormat {
			return fmt.Sprintf("%s needs store format %d", f, spec.DueFormat)
		}
	}
	if format >= spec.DueFormat {
		for _, f := range entry.ControlFreeFields {
			if entry.HasControl(en.Fields.String(f)) {
				return f + " " + entry.ControlMessage
			}
		}
	}
	return ""
}

func nullable(s string) jsonx.Value {
	if s == "" {
		return jsonx.Null
	}
	return jsonx.String(s)
}

var (
	// titleDate is a title that begins with a date not followed by a letter or digit.
	titleDate = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})(?:[^\p{L}\p{N}]|$)`)
	// deadlineDate is a deadline word, then an optional colon, blanks, at most two * or _,
	// and a date (SPEC §11.18). The word must not follow a letter or digit and the date must
	// not be followed by a digit; firstDeadline checks both, so that a rejected match does
	// not consume the text of the next one.
	deadlineDate = regexp.MustCompile(`(?i)(?:due|by|deadline|no later than|on or after):?[ \t]+[*_]{0,2}([0-9]{4}-[0-9]{2}-[0-9]{2})`)
)

// textDate returns the first date a task names in its text as a deadline (SPEC §11.18, a
// date in the text), or "".
func textDate(en *store.Entry) string {
	title := en.Fields.String("title")
	if m := titleDate.FindStringSubmatch(title); m != nil && validDate(m[1]) {
		return m[1]
	}
	if d := firstDeadline(title); d != "" {
		return d
	}
	lines := entry.Lines(en.Body)
	fenced := entry.Fenced(lines)
	for i, l := range lines {
		if fenced[i] || (entry.IsATXHeading(l) && datedHeading.MatchString(l)) {
			continue // code, or a section heading dated as memex update writes it
		}
		if d := firstDeadline(inlineCode.ReplaceAllString(l, " ")); d != "" {
			return d
		}
	}
	return ""
}

var (
	// datedHeading is a heading line that ends in a date, the form of memex update's sections.
	datedHeading = regexp.MustCompile(`[ \t][0-9]{4}-[0-9]{2}-[0-9]{2}[ \t]*$`)
	// inlineCode is text between two backticks, an inline code span.
	inlineCode = regexp.MustCompile("`[^`]*`")
)

// firstDeadline returns the first valid date after a deadline word in s, or "".
func firstDeadline(s string) string {
	for _, m := range deadlineDate.FindAllStringSubmatchIndex(s, -1) {
		if r, _ := utf8.DecodeLastRuneInString(s[:m[0]]); m[0] > 0 && (unicode.IsLetter(r) || unicode.IsNumber(r)) {
			continue
		}
		if m[3] < len(s) && s[m[3]] >= '0' && s[m[3]] <= '9' {
			continue
		}
		if d := s[m[2]:m[3]]; validDate(d) {
			return d
		}
	}
	return ""
}

func validDate(s string) bool {
	_, ok := config.ParseDate(s)
	return ok
}
