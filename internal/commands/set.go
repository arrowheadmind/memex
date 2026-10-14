package commands

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

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

// Set runs `memex set` (SPEC §11.7).
func Set(e *Env, o SetOptions) (int, *UsageError) {
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	entries := w.entries()
	res := newResolver(entries)
	en := res.resolve(e, o.ID)
	if en == nil {
		w.end(nil)
		return spec.ExitFailure, nil
	}
	if why := rewritable(en); why != "" {
		return w.refuse(spec.ExitFailure, "%s has fields that set cannot keep (%s); edit the file, then memex save", en.Path, why), nil
	}
	if code, refused := w.refuseUncommitted(en.Path); refused {
		return code, nil
	}
	f := entry.Fields{}
	for k, v := range en.Fields {
		f[k] = v
	}
	var texts []string
	explicitBy := false
	for _, a := range o.Assignments {
		def := a.field
		switch {
		case a.Field == "created" || a.Field == "body_sha256":
			return w.refuse(spec.ExitFailure, "%s cannot be set: created is fixed by the id, and body_sha256 by the immutable journal body", a.Field), nil
		case !def.AllowedOnType(en.Type.Name):
			return w.refuse(spec.ExitFailure, "field %s is not allowed on a %s; it is allowed on: %s",
				entry.Quote(a.Field), en.Type.Name, strings.Join(def.AllowedOn, ", ")), nil
		case a.Field == "status" && a.Value != "" && !slices.Contains(en.Type.Statuses, a.Value):
			w.end(nil)
			return 0, &UsageError{unknownStatus(en.Type, a.Value)}
		}
		value := a.Value
		if a.Field == "superseded_by" {
			explicitBy = true
		}
		if a.Op == "=" && value == "" {
			if def.Required == "yes" {
				return w.refuse(spec.ExitFailure, "%s is required and cannot be removed", a.Field), nil
			}
			delete(f, a.Field)
			texts = append(texts, a.Field+"=")
			continue
		}
		resolveID := func(v string) (string, bool) {
			if a.Field != "superseded_by" && a.Field != "parent" && a.Field != "sources" {
				return v, true
			}
			t := res.resolve(e, v)
			if t == nil {
				return "", false
			}
			return t.ID, true
		}
		switch def.Kind {
		case spec.KindString:
			id, ok := resolveID(value)
			if !ok {
				w.end(nil)
				return spec.ExitFailure, nil
			}
			f[a.Field], value = id, id
		case spec.KindDate:
			d, ok := config.ParseDate(value)
			if !ok {
				return w.refuse(spec.ExitFailure, "%s must be a date written YYYY-MM-DD, not %s", a.Field, entry.Quote(value)), nil
			}
			f[a.Field] = d
		case spec.KindInteger:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || strings.HasPrefix(value, "+") {
				return w.refuse(spec.ExitFailure, "%s must be an integer, not %s", a.Field, entry.Quote(value)), nil
			}
			f[a.Field] = n
		case spec.KindBoolean:
			switch value {
			case "true":
				f[a.Field] = true
			case "false":
				delete(f, a.Field)
			default:
				return w.refuse(spec.ExitFailure, "%s must be true or false, not %s", a.Field, entry.Quote(value)), nil
			}
		case spec.KindArray:
			items := []string{value}
			if a.Op == "=" {
				items = strings.Split(value, ",")
			}
			for i, it := range items {
				id, ok := resolveID(it)
				if !ok {
					w.end(nil)
					return spec.ExitFailure, nil
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
	data := []byte(entry.Render(f, en.Body))
	if string(data) == string(en.Data) {
		return w.nothing(), nil
	}
	files := []write.File{{Path: en.Path, Data: data, Check: true, Expect: en.Data}}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return e.failedValidation(err), nil
	}
	return w.commit(files, "memex: set "+en.ID+" "+strings.Join(texts, " ")), nil
}
