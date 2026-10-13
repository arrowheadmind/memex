// Package entry parses and writes memex entries (SPEC §5): the file structure, frontmatter
// fields, the canonical form, body normalization and the content measure.
package entry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/arrowheadmind/memex/internal/spec"
)

// Problem is one validation finding about a single file: a SPEC §7 code and its message.
type Problem struct {
	Code    string
	Message string
}

func problem(code, format string, args ...any) Problem {
	return Problem{code, fmt.Sprintf(format, args...)}
}

// Split is an entry file divided as SPEC §5.1 says.
type Split struct {
	Frontmatter string // the lines between the delimiters, each with its LF
	BodyRegion  string // everything after the closing delimiter's LF
	Body        string // BodyRegion without one leading LF
	FrontLine   int    // the file line number of the first frontmatter line
}

// CheckEncoding returns the E-ENCODING problems of data: invalid UTF-8, a BOM, CR.
func CheckEncoding(data []byte) []Problem {
	var ps []Problem
	if !utf8.Valid(data) {
		ps = append(ps, problem("E-ENCODING", "not valid UTF-8"))
	}
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		ps = append(ps, problem("E-ENCODING", "begins with a byte-order mark"))
	}
	if bytes.IndexByte(data, '\r') >= 0 {
		ps = append(ps, problem("E-ENCODING", "contains CR characters; use LF line endings"))
	}
	return ps
}

// SplitFile divides data into frontmatter and body. A missing delimiter is an E-FRONTMATTER
// problem.
func SplitFile(data string) (*Split, *Problem) {
	if !strings.HasPrefix(data, "+++\n") {
		p := problem("E-FRONTMATTER", "the file must begin with a line +++")
		return nil, &p
	}
	rest := data[len("+++\n"):]
	off := 0
	for off <= len(rest) {
		line, _, found := strings.Cut(rest[off:], "\n")
		if line == "+++" {
			s := &Split{Frontmatter: rest[:off], FrontLine: 2}
			if found {
				s.BodyRegion = rest[off+len("+++\n"):]
			}
			s.Body = strings.TrimPrefix(s.BodyRegion, "\n")
			return s, nil
		}
		if !found {
			break
		}
		off += len(line) + 1
	}
	p := problem("E-FRONTMATTER", "no closing +++ line after the frontmatter")
	return nil, &p
}

// Fields are the validated frontmatter values of one entry, keyed by §5.3 name. Values are
// string, time.Time (dates), int64, bool or []string.
type Fields map[string]any

// String returns the string field name, or "".
func (f Fields) String(name string) string {
	s, _ := f[name].(string)
	return s
}

// Strings returns the array field name, or nil.
func (f Fields) Strings(name string) []string {
	s, _ := f[name].([]string)
	return s
}

// Date returns the date field name and whether it is present.
func (f Fields) Date(name string) (time.Time, bool) {
	t, ok := f[name].(time.Time)
	return t, ok
}

// Has reports whether the field is present.
func (f Fields) Has(name string) bool {
	_, ok := f[name]
	return ok
}

var tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// sortedArrays are stored sorted by code point and deduplicated (SPEC §5.1).
var sortedArrays = []string{"tags", "keywords", "repos", "hosts"}

// tomlKind is the toml.MetaData type name each §5.1 kind decodes from.
var tomlKind = map[string]string{
	spec.KindString:  "String",
	spec.KindDate:    "Local Date",
	spec.KindInteger: "Integer",
	spec.KindBoolean: "Bool",
	spec.KindArray:   "Array",
}

var kindText = map[string]string{
	spec.KindString:  "a string",
	spec.KindDate:    "a date written YYYY-MM-DD, unquoted",
	spec.KindInteger: "an integer",
	spec.KindBoolean: "a boolean",
	spec.KindArray:   "an array of strings",
}

// ParseFields decodes the frontmatter of an entry of type typ and checks every field against
// SPEC §5.3: names, the types they are allowed on, kinds, value constraints, required fields,
// the status and the superseded pairing. It returns the fields that are well-formed (so that
// later checks can use them) and every problem found. An invalid TOML document is a single
// E-FRONTMATTER problem and no fields.
func ParseFields(s *Split, typ *spec.Type) (Fields, []Problem) {
	var raw map[string]any
	_, err := toml.Decode(s.Frontmatter, &raw)
	if err != nil {
		msg := "invalid TOML in the frontmatter"
		var pe toml.ParseError
		if errors.As(err, &pe) {
			msg += fmt.Sprintf(" at line %d", pe.Position.Line+s.FrontLine-1)
		}
		return nil, []Problem{problem("E-FRONTMATTER", "%s", msg)}
	}
	var ps []Problem
	fields := Fields{}
	names := make([]string, 0, len(raw))
	for k := range raw {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, name := range names {
		def := spec.LookupField(name)
		if def == nil {
			msg := fmt.Sprintf("unknown field %q", name)
			if near, ok := spec.Nearest(name, spec.FieldNames()); ok {
				msg += fmt.Sprintf(" (did you mean %q?)", near)
			}
			ps = append(ps, Problem{"E-FIELD-UNKNOWN", msg})
			continue
		}
		if !def.AllowedOnType(typ.Name) {
			ps = append(ps, problem("E-FIELD-UNKNOWN", "field %q is not allowed on a %s; it is allowed on: %s",
				name, typ.Name, strings.Join(def.AllowedOn, ", ")))
			continue
		}
		v, p := fieldValue(def, raw[name])
		if p != nil {
			ps = append(ps, *p)
			continue
		}
		fields[name] = v
	}
	for _, def := range spec.Fields {
		if def.Required == "yes" && def.AllowedOnType(typ.Name) && raw[def.Name] == nil {
			ps = append(ps, problem("E-FIELD-MISSING", "missing required field %q", def.Name))
		}
	}
	if st, ok := fields["status"].(string); ok && !slices.Contains(typ.Statuses, st) {
		msg := fmt.Sprintf("status %q is not valid for a %s", st, typ.Name)
		if near, ok := spec.Nearest(st, typ.Statuses); ok {
			msg += fmt.Sprintf(" (did you mean %q?)", near)
		}
		ps = append(ps, problem("E-STATUS", "%s; valid: %s", msg, strings.Join(typ.Statuses, ", ")))
	}
	if st, ok := fields["status"].(string); ok {
		_, hasBy := raw["superseded_by"]
		switch {
		case st == "superseded" && !hasBy:
			ps = append(ps, problem("E-SUPERSEDED", "status is superseded but superseded_by is missing"))
		case st != "superseded" && hasBy:
			ps = append(ps, problem("E-SUPERSEDED", "superseded_by is set but status is %q, not superseded", st))
		}
	}
	return fields, ps
}

// tomlKindOf names the TOML kind of a decoded value. BurntSushi/toml decodes every date and
// time kind into time.Time and marks a Local Date only by the name of its location
// (IMPLEMENTATION.md, "TOML"); TestLocalDateLocation pins that.
func tomlKindOf(v any) string {
	switch v := v.(type) {
	case string:
		return "String"
	case int64:
		return "Integer"
	case bool:
		return "Bool"
	case []any:
		return "Array"
	case time.Time:
		if v.Location().String() == "date-local" {
			return "Local Date"
		}
		return "Datetime"
	}
	return "other"
}

func fieldValue(def *spec.Field, v any) (any, *Problem) {
	bad := func(format string, args ...any) (any, *Problem) {
		p := problem("E-FIELD-VALUE", "%s %s", def.Name, fmt.Sprintf(format, args...))
		return nil, &p
	}
	if tomlKindOf(v) != tomlKind[def.Kind] {
		return bad("must be %s", kindText[def.Kind])
	}
	switch def.Kind {
	case spec.KindString:
		s := v.(string)
		switch def.Name {
		case "title":
			if n := utf8.RuneCountInString(s); n < 1 || n > 300 || strings.Contains(s, "\n") {
				return bad("must be 1 to 300 characters on one line")
			}
		case "summary":
			if utf8.RuneCountInString(s) > 500 || strings.Contains(s, "\n") {
				return bad("must be one line of at most 500 characters")
			}
		case "kind":
			if s != "prompt" && s != "report" {
				return bad("must be \"prompt\" or \"report\", not %q", s)
			}
		case "body_sha256":
			if !hexPattern.MatchString(s) {
				return bad("must be 64 lowercase hex digits")
			}
		}
		return s, nil
	case spec.KindDate:
		y, m, d := v.(time.Time).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	case spec.KindInteger:
		n := v.(int64)
		if n < 1 {
			return bad("must be at least 1, not %d", n)
		}
		return n, nil
	case spec.KindBoolean:
		return v.(bool), nil
	case spec.KindArray:
		items, _ := v.([]any)
		out := make([]string, 0, len(items))
		for _, it := range items {
			s, ok := it.(string)
			if !ok {
				return bad("must be %s", kindText[def.Kind])
			}
			switch {
			case def.Name == "tags" && !tagPattern.MatchString(s):
				return bad("element %q does not match ^[a-z0-9][a-z0-9-]*$", s)
			case strings.Contains(s, "\n"):
				return bad("element %q contains a line feed", s)
			}
			out = append(out, s)
		}
		return out, nil
	}
	panic("unknown kind " + def.Kind)
}

// Render returns the canonical form of an entry (SPEC §5.1): fields in §5.3 order, arrays
// sorted where §5.1 says, empty arrays and false booleans omitted, then the body as given.
func Render(fields Fields, body string) string {
	var b strings.Builder
	b.WriteString("+++\n")
	for _, def := range spec.Fields {
		v, ok := fields[def.Name]
		if !ok {
			continue
		}
		switch v := v.(type) {
		case string:
			b.WriteString(def.Name + " = " + quote(v) + "\n")
		case time.Time:
			b.WriteString(def.Name + " = " + v.Format(time.DateOnly) + "\n")
		case int64:
			fmt.Fprintf(&b, "%s = %d\n", def.Name, v)
		case bool:
			if v {
				b.WriteString(def.Name + " = true\n")
			}
		case []string:
			if slices.Contains(sortedArrays, def.Name) {
				v = SortedSet(v)
			}
			if len(v) == 0 {
				continue
			}
			quoted := make([]string, len(v))
			for i, s := range v {
				quoted[i] = quote(s)
			}
			b.WriteString(def.Name + " = [" + strings.Join(quoted, ", ") + "]\n")
		}
	}
	b.WriteString("+++\n\n")
	b.WriteString(body)
	return b.String()
}

// SortedSet returns ss sorted by code point with duplicates removed.
func SortedSet(ss []string) []string {
	out := slices.Clone(ss)
	slices.Sort(out) // byte order of UTF-8 is code point order
	return slices.Compact(out)
}

// quote renders s as a TOML basic string (SPEC §5.1).
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// CanonicalBody returns the body as the canonical form holds it: normalized, except for a
// journal, whose bytes are what body_sha256 covers (SPEC §5.1).
func CanonicalBody(typ *spec.Type, body string) string {
	if typ.Name == "journal" {
		return body
	}
	return Normalize(body)
}

// Normalize applies the body normalization of SPEC §5.1: CRLF and CR become LF, trailing
// whitespace is removed, and exactly one LF is appended. Whitespace is the Unicode
// White_Space property.
func Normalize(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.TrimRightFunc(body, unicode.IsSpace) + "\n"
}

// BodySHA256 is the lowercase hex SHA-256 of the body bytes.
func BodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// Lines splits a body into lines without their LFs. A final LF does not start another line.
func Lines(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

var fenceOpen = regexp.MustCompile("^ {0,3}(```+|~~~+)")

// Fenced reports, for each line, whether it belongs to a fenced code block, fence lines
// included (SPEC §5.6). An unclosed block runs to the end.
func Fenced(lines []string) []bool {
	in := make([]bool, len(lines))
	fence := ""
	for i, line := range lines {
		if fence == "" {
			if m := fenceOpen.FindStringSubmatch(line); m != nil {
				fence = m[1]
				in[i] = true
			}
			continue
		}
		in[i] = true
		if closesFence(line, fence) {
			fence = ""
		}
	}
	return in
}

func closesFence(line, fence string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return false
	}
	run := len(trimmed) - len(strings.TrimLeft(trimmed, fence[:1]))
	return run >= len(fence) && strings.TrimFunc(trimmed[run:], unicode.IsSpace) == ""
}

var atxHeading = regexp.MustCompile(`^ {0,3}#{1,6}( |$)`)

// ContentMeasure is the number of non-whitespace code points in body lines that are not ATX
// headings; lines inside fenced code blocks always count (SPEC §5.6).
func ContentMeasure(body string) int {
	lines := Lines(body)
	fenced := Fenced(lines)
	n := 0
	for i, line := range lines {
		if !fenced[i] && atxHeading.MatchString(line) {
			continue
		}
		for _, r := range line {
			if !unicode.IsSpace(r) {
				n++
			}
		}
	}
	return n
}

// MinContent is the smallest content measure a new body may have (SPEC §5.6).
const MinContent = 80
