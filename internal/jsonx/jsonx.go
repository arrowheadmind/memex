// Package jsonx writes JSON exactly as SPEC §10 requires: compact, keys in the order given,
// and strings escaped only where §10 says. encoding/json cannot do this (it escapes <, >, &,
// U+2028 and U+2029, and writes \b and \f), so values are built explicitly and never by
// reflection.
package jsonx

import (
	"strconv"
	"unicode/utf8"
)

// Value is any JSON value this package can write.
type Value interface {
	appendTo(b []byte) []byte
}

// Member is one key and value of an Object.
type Member struct {
	Key   string
	Value Value
}

// Object is a JSON object whose members are written in slice order.
type Object []Member

// Array is a JSON array.
type Array []Value

// String is a JSON string.
type String string

// Int is a JSON integer.
type Int int64

// Bool is a JSON boolean.
type Bool bool

// Score is the one non-integer number memex prints (SPEC §8.5): fixed notation with exactly
// six digits after the decimal point, rounded half to even on the exact binary value.
// Callers MUST apply the §8.3 rounding to the score first (IMPLEMENTATION.md, "Rounding");
// Score only formats the value it is given.
type Score float64

// Null is the JSON null.
var Null Value = null{}

type null struct{}

// Strings converts a string slice to an Array. A nil slice becomes [], never null.
func Strings(ss []string) Array {
	a := make(Array, len(ss))
	for i, s := range ss {
		a[i] = String(s)
	}
	return a
}

// StringOrNull is null when p is nil, else the string *p.
func StringOrNull(p *string) Value {
	if p == nil {
		return Null
	}
	return String(*p)
}

// Encode returns v as one compact JSON text followed by one LF.
func Encode(v Value) []byte {
	return append(v.appendTo(nil), '\n')
}

func (o Object) appendTo(b []byte) []byte {
	b = append(b, '{')
	for i, m := range o {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendString(b, m.Key)
		b = append(b, ':')
		b = m.Value.appendTo(b)
	}
	return append(b, '}')
}

func (a Array) appendTo(b []byte) []byte {
	b = append(b, '[')
	for i, v := range a {
		if i > 0 {
			b = append(b, ',')
		}
		b = v.appendTo(b)
	}
	return append(b, ']')
}

func (s String) appendTo(b []byte) []byte { return appendString(b, string(s)) }

func (n Int) appendTo(b []byte) []byte { return strconv.AppendInt(b, int64(n), 10) }

func (v Bool) appendTo(b []byte) []byte { return strconv.AppendBool(b, bool(v)) }

func (s Score) appendTo(b []byte) []byte { return strconv.AppendFloat(b, float64(s), 'f', 6, 64) }

func (null) appendTo(b []byte) []byte { return append(b, "null"...) }

const hex = "0123456789abcdef"

// appendString escapes exactly ", \, LF, CR, TAB and the other code points below U+0020
// (as \u00xx, lowercase). Everything else is literal. A byte that is not valid UTF-8 is
// written as U+FFFD, so the output is always valid UTF-8.
func appendString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				b = append(b, "�"...)
			} else {
				b = append(b, s[i:i+size]...)
			}
			i += size
			continue
		}
		switch {
		case c == '"':
			b = append(b, '\\', '"')
		case c == '\\':
			b = append(b, '\\', '\\')
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c == '\t':
			b = append(b, '\\', 't')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
		default:
			b = append(b, c)
		}
		i++
	}
	return append(b, '"')
}
