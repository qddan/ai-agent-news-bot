package textutil

import "testing"

func TestPlain_StripsTagsAndEntities(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"<p>Hello <b>world</b></p>": "Hello world",
		"a &amp; b":                 "a & b",
		"x&amp;#x27;s":              "x's", // double-encoded apostrophe
		"&lt;a href=&#34;u&#34;&gt;link&lt;/a&gt;": "link",        // entity-encoded tag revealed then stripped
		"line1\n\n  line2":                         "line1 line2", // whitespace collapse
	}
	for in, want := range cases {
		if got := Plain(in); got != want {
			t.Errorf("Plain(%q) = %q, want %q", in, got, want)
		}
	}
}
