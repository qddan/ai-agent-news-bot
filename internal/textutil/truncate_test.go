package textutil

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateRunes_NoUTF8Corruption(t *testing.T) {
	s := "nội dung tiếng Việt với nhiều ký tự đa byte ềếệ"
	for n := 0; n <= utf8.RuneCountInString(s)+2; n++ {
		got := TruncateRunes(s, n)
		if !utf8.ValidString(got) {
			t.Errorf("TruncateRunes(%q, %d) produced invalid UTF-8: %q", s, n, got)
		}
		if rc := utf8.RuneCountInString(got); rc > n {
			t.Errorf("TruncateRunes returned %d runes, want <= %d", rc, n)
		}
	}
}

func TestTruncateRunes_ShortStringUnchanged(t *testing.T) {
	if got := TruncateRunes("abc", 10); got != "abc" {
		t.Errorf("got %q, want abc", got)
	}
}
