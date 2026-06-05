// Package textutil normalizes source-provided snippets into plain text.
// Feeds (HN story_text, RSS/arXiv descriptions, Reddit selftext) often contain
// HTML tags and entities; we strip them so summaries are clean both in the LLM
// prompt and in the raw fallback digest.
package textutil

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	wsRe    = regexp.MustCompile(`\s+`)
	maxRuns = 2 // collapse runs of entities/whitespace defensively
)

// TruncateRunes returns at most n runes of s, cutting on a rune boundary so
// multibyte text (e.g. Vietnamese) is never corrupted. A naive s[:n] byte slice
// can split a multibyte rune and emit invalid UTF-8.
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// Plain strips HTML tags, unescapes entities (twice, since feeds sometimes
// double-encode), and collapses whitespace. Returns trimmed plain text.
func Plain(s string) string {
	if s == "" {
		return ""
	}
	s = tagRe.ReplaceAllString(s, " ")
	for i := 0; i < maxRuns; i++ {
		s = html.UnescapeString(s)
	}
	// drop any tags revealed after unescaping (e.g. &lt;a&gt; -> <a>)
	s = tagRe.ReplaceAllString(s, " ")
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
