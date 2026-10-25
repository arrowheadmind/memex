package commands

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/arrowheadmind/memex/internal/check"
	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
	"github.com/arrowheadmind/memex/internal/write"
)

// Assignment is one argument of `memex set` (SPEC §11.7).
type Assignment struct {
	Field string
	Op    string // "=", "+=" or "-="
	Value string
	field *spec.Field
}

// ParseAssignment parses field=value, field=, field+=value or field-=value. A malformed
// assignment or an unknown field is a usage problem.
func ParseAssignment(arg string) (Assignment, string) {
	name, value, ok := strings.Cut(arg, "=")
	if !ok {
		return Assignment{}, fmt.Sprintf("assignment %q has no \"=\"; write field=value, field=, field+=value or field-=value", arg)
	}
	a := Assignment{Field: name, Op: "=", Value: value}
	if n, ok := strings.CutSuffix(name, "+"); ok {
		a.Field, a.Op = n, "+="
	} else if n, ok := strings.CutSuffix(name, "-"); ok {
		a.Field, a.Op = n, "-="
	}
	a.field = spec.LookupField(a.Field)
	if a.field == nil {
		problem := fmt.Sprintf("unknown field %q", a.Field)
		if near, ok := spec.Nearest(a.Field, spec.FieldNames()); ok {
			return a, problem + fmt.Sprintf("; did you mean %q?", near)
		}
		return a, problem + "; valid: " + strings.Join(spec.FieldNames(), ", ")
	}
	if a.Op != "=" && a.field.Kind != spec.KindArray {
		return a, fmt.Sprintf("%s%s works only on array fields; %s is a %s, write %s=VALUE", a.Field, a.Op, a.Field, a.field.Kind, a.Field)
	}
	return a, ""
}

// SetOptions are the arguments of `memex set`.
type SetOptions struct {
	ID          string
	Assignments []Assignment
}

// cutValue shortens a commit-message value to 40 code points (SPEC §9.2).
func cutValue(v string) string {
	if utf8.RuneCountInString(v) <= 40 {
		return v
	}
	r := []rune(v)
	return string(r[:39]) + "…"
}

// rewritable reports why an entry's fields cannot be rewritten without losing a value: an
// unknown field, or one whose value is not valid. Rendering keeps only valid known fields.
func rewritable(en *store.Entry) string {
	split, p := entry.SplitFile(string(en.Data))
	if p != nil {
		return p.Message
	}
	_, ps := entry.ParseFields(split, en.Type, en.ID)
	var bad []string
	for _, p := range ps {
		if p.Code == "E-FIELD-UNKNOWN" || p.Code == "E-FIELD-VALUE" {
			bad = append(bad, p.Message)
		}
	}
	return strings.Join(bad, "; ")
}

// refusal is why one set cannot be made: the diagnostic without the "memex: " prefix, and
// whether the single form reports it as a usage error (SPEC §16.8).
type refusal struct {
	msg   string
	usage bool
}

func refused(format string, args ...any) *refusal {
	return &refusal{msg: fmt.Sprintf(format, args...)}
}

// planSet computes the new contents of en with the assignments applied (SPEC §11.7), and the
// text of the assignments for the commit message, with ids as resolved. It writes nothing.
func planSet(w *writing, res *resolver, en *store.Entry, asgs []Assignment) ([]byte, string, *refusal) {
	f, texts, r := planFields(w, res, en, asgs)
	if r != nil {
		return nil, "", r
	}
	return []byte(entry.Render(f, en.Body)), "memex: set " + en.ID + " " + strings.Join(texts, " "), nil
}

// crossesVisibility reports whether a status change from old to new closes or reopens an
// entry of type typ: exactly one of the two is visible by default (SPEC §5.2).
func crossesVisibility(typ *spec.Type, old, new string) bool {
	return slices.Contains(typ.Visible, old) != slices.Contains(typ.Visible, new)
}

// planFields computes the fields of en with the assignments applied, and the text of each
// assignment for the commit message (SPEC §11.7).
func planFields(w *writing, res *resolver, en *store.Entry, asgs []Assignment) (entry.Fields, []string, *refusal) {
	if w.s.Uncommitted(en.Path) {
		return nil, nil, refused("%s has uncommitted changes; run memex save first", en.Path)
	}
	why, unusable := w.unusable[en.Path]
	if !unusable {
		why = rewritable(en)
	}
	if why != "" {
		return nil, nil, refused("%s has fields that set cannot keep (%s); edit the file, then memex save", en.Path, why)
	}
	f := entry.Fields{}
	for k, v := range en.Fields {
		f[k] = v
	}
	var texts []string
	explicitBy := false
	for _, a := range asgs {
		def := a.field
		switch {
		case a.Field == "created" || a.Field == "body_sha256":
			return nil, nil, refused("%s cannot be set: created is fixed by the id, and body_sha256 by the immutable journal body", a.Field)
		case !def.AllowedOnType(en.Type.Name):
			return nil, nil, refused("field %s is not allowed on a %s; it is allowed on: %s",
				entry.Quote(a.Field), en.Type.Name, strings.Join(def.AllowedOn, ", "))
		case a.Field == "status" && a.Value != "" && !slices.Contains(en.Type.Statuses, a.Value):
			return nil, nil, &refusal{msg: unknownStatus(en.Type, a.Value), usage: true}
		}
		value := a.Value
		if a.Field == "superseded_by" {
			explicitBy = true
		}
		if a.Op == "=" && value == "" {
			if def.Required == "yes" {
				return nil, nil, refused("%s is required and cannot be removed", a.Field)
			}
			delete(f, a.Field)
			texts = append(texts, a.Field+"=")
			continue
		}
		resolveID := func(v string) (string, *refusal) {
			if a.Field != "superseded_by" && a.Field != "parent" && a.Field != "sources" {
				return v, nil
			}
			if t, _ := res.lookup(v); t != nil {
				return t.ID, nil
			}
			return "", &refusal{msg: res.explain(v)}
		}
		switch def.Kind {
		case spec.KindString:
			id, r := resolveID(value)
			if r != nil {
				return nil, nil, r
			}
			f[a.Field], value = id, id
		case spec.KindDate:
			d, ok := config.ParseDate(value)
			if !ok {
				return nil, nil, refused("%s must be a date written YYYY-MM-DD, not %s", a.Field, entry.Quote(value))
			}
			f[a.Field] = d
		case spec.KindInteger:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || strings.HasPrefix(value, "+") {
				return nil, nil, refused("%s must be an integer, not %s", a.Field, entry.Quote(value))
			}
			f[a.Field] = n
		case spec.KindBoolean:
			switch value {
			case "true":
				f[a.Field] = true
			case "false":
				delete(f, a.Field)
			default:
				return nil, nil, refused("%s must be true or false, not %s", a.Field, entry.Quote(value))
			}
		case spec.KindArray:
			items := []string{value}
			if a.Op == "=" {
				items = strings.Split(value, ",")
			}
			for i, it := range items {
				id, r := resolveID(it)
				if r != nil {
					return nil, nil, r
				}
				items[i] = id
			}
			value = strings.Join(items, ",")
			cur := slices.Clone(f.Strings(a.Field))
			switch a.Op {
			case "=":
				cur = nil
				for _, it := range items {
					if !slices.Contains(cur, it) {
						cur = append(cur, it)
					}
				}
			case "+=":
				if !slices.Contains(cur, items[0]) {
					cur = append(cur, items[0])
				}
			case "-=":
				cur = slices.DeleteFunc(cur, func(s string) bool { return s == items[0] })
			}
			if len(cur) == 0 {
				delete(f, a.Field)
			} else {
				f[a.Field] = cur
			}
		}
		texts = append(texts, a.Field+a.Op+cutValue(value))
	}
	// Leaving superseded removes superseded_by (SPEC §11.7).
	if en.Status() == "superseded" && f.String("status") != "superseded" && !explicitBy {
		delete(f, "superseded_by")
	}
	// A status change that closes or reopens a task or plan needs a new summary (SPEC §11.7).
	if old, new := en.Status(), f.String("status"); old != new && slices.Contains(check.CurrencyTypes, en.Type.Name) &&
		crossesVisibility(en.Type, old, new) && (f.String("summary") == "" || f.String("summary") == en.Fields.String("summary")) {
		return nil, nil, refused("%s changes status from %s to %s but keeps its summary; add summary=<sentence> to the same command, "+
			"saying where it stands now", en.ID, entry.Quote(old), entry.Quote(new))
	}
	return f, texts, nil
}

// Set runs `memex set ID ASSIGNMENT…` (SPEC §11.7).
func Set(e *Env, o SetOptions) (int, *UsageError) {
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	res := newResolver(w.entries())
	en, r := w.target(res, o.ID)
	if r != nil {
		return w.refuse(spec.ExitFailure, "%s", r.msg), nil
	}
	data, msg, r := planSet(w, res, en, o.Assignments)
	if r != nil {
		w.end(nil)
		if r.usage {
			return 0, &UsageError{r.msg}
		}
		return e.fail(spec.ExitFailure, "%s", r.msg), nil
	}
	if string(data) == string(en.Data) {
		return w.nothing(), nil
	}
	files := []write.File{{Path: en.Path, Data: data, Check: true, Expect: en.Data}}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return e.failedValidation(err), nil
	}
	return w.commit(files, msg), nil
}

// SetBatch runs `memex set --batch FILE` (SPEC §11.7): every line of FILE is one set, and the
// whole batch is one §9.1 unit with one commit. File is a path, or "-" for standard input.
func SetBatch(e *Env, file string) (int, *UsageError) {
	text, err := e.readBody(file)
	if err != nil {
		return 0, &UsageError{fmt.Sprintf("cannot read --batch %s: %v", file, unwrap(err))}
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	res := newResolver(w.entries())
	lineOf := map[string]int{} // entry path -> the line that names it
	var files []write.File
	var body []string
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		fail := func(msg string) (int, *UsageError) {
			w.end(nil)
			return e.fail(spec.ExitFailure, "--batch line %d: %s", n, msg), nil
		}
		line = strings.TrimSuffix(line, "\r")
		if t := strings.TrimLeft(line, " \t"); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		words, problem := splitWords(line)
		if problem != "" {
			return fail(problem)
		}
		if len(words) == 1 {
			return fail("missing ASSIGNMENT; write field=value, field=, field+=value or field-=value")
		}
		var asgs []Assignment
		for _, arg := range words[1:] {
			a, problem := ParseAssignment(arg)
			if problem != "" {
				return fail(problem)
			}
			asgs = append(asgs, a)
		}
		en, r := w.target(res, words[0])
		if r != nil {
			return fail(r.msg)
		}
		if m, dup := lineOf[en.Path]; dup {
			return fail(fmt.Sprintf("%s is already set by line %d; put all its assignments on one line", en.Path, m))
		}
		lineOf[en.Path] = n
		data, msg, r := planSet(w, res, en, asgs)
		if r != nil {
			return fail(r.msg)
		}
		if string(data) != string(en.Data) {
			files = append(files, write.File{Path: en.Path, Data: data, Check: true, Expect: en.Data})
			body = append(body, msg)
		}
	}
	if len(files) == 0 {
		return w.nothing(), nil
	}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		ve, ok := err.(*write.ValidationError)
		if !ok {
			panic(err)
		}
		for _, f := range ve.Findings {
			prefix := ""
			if n, ok := lineOf[f.Path]; ok {
				prefix = fmt.Sprintf("--batch line %d: ", n)
			}
			fmt.Fprintf(e.Stderr, "memex: %s%s: %s %s\n", prefix, f.Path, f.Code, f.Message)
		}
		return e.fail(spec.ExitFailure, "refused: the result would not validate; nothing was changed"), nil
	}
	msg := fmt.Sprintf("memex: set %d entries\n\n%s", len(files), strings.Join(body, "\n"))
	return w.commit(files, msg), nil
}

// splitWords splits a --batch line like a POSIX shell without expansions (SPEC §11.7):
// single quotes are literal, double quotes allow \" and \\, and a backslash outside quotes
// escapes the next character. Words are separated by spaces and tabs.
func splitWords(line string) ([]string, string) {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				return nil, "unterminated single quote"
			}
			cur.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord = true
		case ch == '"':
			i++
			for ; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) && (line[i+1] == '"' || line[i+1] == '\\') {
					i++
				}
				cur.WriteByte(line[i])
			}
			if i >= len(line) {
				return nil, "unterminated double quote"
			}
			inWord = true
		case ch == '\\' && i+1 < len(line):
			i++
			cur.WriteByte(line[i])
			inWord = true
		case ch == ' ' || ch == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(ch)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, ""
}
