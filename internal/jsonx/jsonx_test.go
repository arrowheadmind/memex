package jsonx

import (
	"encoding/json"
	"fmt"
	"testing"
)

func enc(v Value) string { return string(Encode(v)) }

func TestStringEscapes(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", `""`},
		{"plain", "abc", `"abc"`},
		{"quote", `a"b`, `"a\"b"`},
		{"backslash", `a\b`, `"a\\b"`},
		{"LF", "a\nb", `"a\nb"`},
		{"CR", "a\rb", `"a\rb"`},
		{"TAB", "a\tb", `"a\tb"`},
		{"NUL", "\x00", `"\u0000"`},
		{"BS is not \\b", "\b", `"\u0008"`},
		{"FF is not \\f", "\f", `"\u000c"`},
		{"VT", "\v", `"\u000b"`},
		{"ESC lowercase hex", "\x1b", `"\u001b"`},
		{"US", "\x1f", `"\u001f"`},
		{"space", " ", `" "`},
		{"slash literal", "a/b", `"a/b"`},
		{"html literal", "<a>&", `"<a>&"`},
		{"DEL literal", "\x7f", "\"\x7f\""},
		{"U+0080 literal", "\u0080", "\"\u0080\""},
		{"U+2028 literal", " ", "\" \""},
		{"U+2029 literal", " ", "\" \""},
		{"non-ASCII literal", "Café ﬁ — …", `"Café ﬁ — …"`},
		{"astral literal", "😀", `"😀"`},
		{"invalid byte", "a\xffb", "\"a�b\""},
		{"truncated sequence", "\xe2\x80", "\"��\""},
		{"U+FFFD literal", "�", "\"�\""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := enc(String(c.in)); got != c.want+"\n" {
				t.Errorf("String(%q) = %q, want %q", c.in, got, c.want+"\n")
			}
		})
	}
}

// Every code point below U+0020 other than LF, CR and TAB is \u00xx with lowercase hex.
func TestAllControlCharacters(t *testing.T) {
	for c := 0; c < 0x20; c++ {
		var want string
		switch c {
		case '\n':
			want = `"\n"`
		case '\r':
			want = `"\r"`
		case '\t':
			want = `"\t"`
		default:
			want = fmt.Sprintf(`"\u%04x"`, c)
		}
		if got := enc(String(string(rune(c)))); got != want+"\n" {
			t.Errorf("U+%04X: got %q, want %q", c, got, want)
		}
	}
}

// Whatever is written must still be JSON that a standard parser reads back unchanged.
func TestRoundTrip(t *testing.T) {
	var s []byte
	for c := 0; c < 0x80; c++ {
		s = append(s, byte(c))
	}
	s = append(s, "  Café😀"...)
	var back string
	if err := json.Unmarshal(Encode(String(s)), &back); err != nil {
		t.Fatal(err)
	}
	if back != string(s) {
		t.Errorf("round trip changed the string: %q", back)
	}
}

func TestCompositeOrderAndCompactness(t *testing.T) {
	v := Object{
		{"z", Int(1)},
		{"a", Array{String("x"), Null, Bool(true), Bool(false)}},
		{"m", Object{}},
		{"e", Array{}},
		{"n", Int(-42)},
		{"s", Strings(nil)},
		{"p", StringOrNull(nil)},
	}
	want := `{"z":1,"a":["x",null,true,false],"m":{},"e":[],"n":-42,"s":[],"p":null}` + "\n"
	if got := enc(v); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	text := "t"
	if got := enc(StringOrNull(&text)); got != "\"t\"\n" {
		t.Errorf("StringOrNull: got %q", got)
	}
}

func TestKeysAreEscaped(t *testing.T) {
	if got := enc(Object{{"a\"\n", Int(0)}}); got != `{"a\"\n":0}`+"\n" {
		t.Errorf("got %q", got)
	}
}

func TestScore(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.000000"},
		{1, "1.000000"},
		{12.345679, "12.345679"},
		{12.3456789, "12.345679"},
		{1e-7, "0.000000"},
		{123456789.5, "123456789.500000"},
		{1.0 / 3.0, "0.333333"},
		// Exact binary ties at the seventh decimal round half to even:
		// 2^-7 = 0.0078125 and 3 × 2^-7 = 0.0234375 are exactly representable.
		{0.0078125, "0.007812"},
		{0.0234375, "0.023438"},
	}
	for _, c := range cases {
		if got := enc(Score(c.in)); got != c.want+"\n" {
			t.Errorf("Score(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
