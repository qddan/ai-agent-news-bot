package deliver

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A single un-newlined line longer than the limit must still be broken so no
// emitted piece exceeds the limit (otherwise Telegram rejects with HTTP 400).
func TestHardSplit_BreaksOverlongSingleLine(t *testing.T) {
	line := strings.Repeat("a", 5021) // one line, no spaces, no newlines
	pieces := hardSplit(line, packLimit)
	if len(pieces) < 2 {
		t.Fatalf("expected the overlong line to be split, got %d piece(s)", len(pieces))
	}
	for i, p := range pieces {
		if len(p) > packLimit {
			t.Errorf("piece %d exceeds packLimit: %d", i, len(p))
		}
	}
}

func TestSplitLine_PrefersWordBoundary(t *testing.T) {
	line := strings.Repeat("word ", 2000) // ~10000 chars, breakable on spaces
	pieces := splitLine(line, 100)
	for i, p := range pieces {
		if len(p) > 100 {
			t.Errorf("piece %d exceeds limit: %d", i, len(p))
		}
	}
	// rejoining should preserve the words (spaces normalized away at breaks)
	if !strings.Contains(strings.Join(pieces, " "), "word word") {
		t.Error("expected words preserved across split")
	}
}

func TestSplitLine_NeverCorruptsUTF8(t *testing.T) {
	line := strings.Repeat("nội dung tiếng Việt ", 500) // multibyte, no ASCII-only run
	pieces := splitLine(line, 73)                       // odd limit to land mid-rune
	for i, p := range pieces {
		if !utf8.ValidString(p) {
			t.Errorf("piece %d is not valid UTF-8", i)
		}
		if len(p) > 73 {
			t.Errorf("piece %d exceeds limit: %d", i, len(p))
		}
	}
}
