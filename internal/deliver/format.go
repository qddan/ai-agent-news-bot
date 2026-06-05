// Package deliver renders the digest into Telegram-ready HTML messages and
// sends them, respecting Telegram's 4096-char/message limit and rate limits.
package deliver

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

// telegramMaxChars is Telegram's hard per-message limit. footerReserve leaves
// room for the "Phần x/y" footer appended to multi-part messages, so a packed
// message never exceeds the hard limit once the footer is added.
const (
	telegramMaxChars = 4096
	footerReserve    = 48
	packLimit        = telegramMaxChars - footerReserve
)

// Format builds the full digest HTML and splits it into chunks that each fit
// under the Telegram limit, never cutting in the middle of an item or tag.
// `items` reference `arts` by Index (for the canonical URL/source).
func Format(items []model.DigestItem, arts []model.Article, now time.Time, loc *time.Location) []string {
	header := fmt.Sprintf("📰 <b>AI Agents — Bản tin %s (09:00 ICT)</b>",
		now.In(loc).Format("02/01/2006"))

	blocks := buildBlocks(items, arts)
	return splitMessages(header, blocks)
}

// EmptyNote is sent when there is no qualifying news, so the user knows the job ran.
func EmptyNote(now time.Time, loc *time.Location) []string {
	return []string{fmt.Sprintf("📰 <b>AI Agents — Bản tin %s</b>\n\nKhông có tin AI-agent nổi bật trong 24h qua.",
		now.In(loc).Format("02/01/2006"))}
}

// buildBlocks groups items by category (in first-seen order), sorts items
// within a category by rank, and renders each category to an HTML block.
func buildBlocks(items []model.DigestItem, arts []model.Article) []string {
	type group struct {
		name  string
		items []model.DigestItem
	}
	var groups []*group
	index := map[string]*group{}

	// Stable category order: by best (lowest) rank seen in each category.
	ordered := make([]model.DigestItem, len(items))
	copy(ordered, items)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Rank < ordered[j].Rank })

	for _, it := range ordered {
		g, ok := index[it.Category]
		if !ok {
			g = &group{name: it.Category}
			index[it.Category] = g
			groups = append(groups, g)
		}
		g.items = append(g.items, it)
	}

	var blocks []string
	for _, g := range groups {
		var b strings.Builder
		fmt.Fprintf(&b, "▍<b>%s</b>\n", html.EscapeString(g.name))
		for _, it := range g.items {
			b.WriteString(renderItem(it, arts))
		}
		blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
	}
	return blocks
}

// renderItem renders one digest entry. Source/URL come from the referenced Article.
func renderItem(it model.DigestItem, arts []model.Article) string {
	source, url := "", ""
	if it.Index >= 0 && it.Index < len(arts) {
		source = arts[it.Index].Source
		url = arts[it.Index].URL
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n<b>%s</b>\n", html.EscapeString(it.TitleVI))
	if s := strings.TrimSpace(it.SummaryVI); s != "" {
		fmt.Fprintf(&b, "%s\n", html.EscapeString(s))
	}
	if url != "" {
		fmt.Fprintf(&b, "🔗 <a href=\"%s\">%s</a>\n", html.EscapeString(url), html.EscapeString(source))
	}
	return b.String()
}

// splitMessages greedily packs blocks (joined by a blank line) into messages
// under the char limit. If multiple messages result, each gets a "Phần x/y"
// footer. A single block larger than the limit is hard-split as a last resort.
func splitMessages(header string, blocks []string) []string {
	if len(blocks) == 0 {
		return []string{header}
	}
	// Keep the header attached to the first block so it never lands in a
	// near-empty standalone message.
	blocks = append([]string{header + "\n\n" + blocks[0]}, blocks[1:]...)

	var messages []string
	current := ""

	flush := func() {
		if strings.TrimSpace(current) != "" {
			messages = append(messages, current)
		}
		current = ""
	}

	for _, block := range blocks {
		candidate := block
		if current != "" {
			candidate = current + "\n\n" + block
		}
		if len(candidate) <= packLimit {
			current = candidate
			continue
		}
		// doesn't fit — flush what we have, then place the block
		flush()
		if len(block) <= packLimit {
			current = block
		} else {
			for _, piece := range hardSplit(block, packLimit) {
				messages = append(messages, piece)
			}
			current = ""
		}
	}
	flush()

	if len(messages) <= 1 {
		return messages
	}
	total := len(messages)
	for i := range messages {
		messages[i] = fmt.Sprintf("%s\n\n<i>Phần %d/%d</i>", messages[i], i+1, total)
	}
	return messages
}

// hardSplit breaks an oversized block on line boundaries as a fallback. A
// single line that is itself longer than limit is further broken by splitLine
// so no emitted piece can exceed limit (which would make Telegram reject it).
func hardSplit(s string, limit int) []string {
	var out []string
	var cur strings.Builder

	emit := func() {
		if cur.Len() > 0 {
			out = append(out, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
		}
	}

	for _, line := range strings.Split(s, "\n") {
		if len(line) > limit {
			emit()
			pieces := splitLine(line, limit)
			out = append(out, pieces[:len(pieces)-1]...)
			line = pieces[len(pieces)-1] // carry the remainder into the running buffer
		}
		if cur.Len()+len(line)+1 > limit && cur.Len() > 0 {
			emit()
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	emit()
	return out
}

// splitLine breaks a single over-long line into <=limit-byte pieces on rune
// boundaries, preferring to break at the last space so words stay intact.
// Note: this only ever runs on lines exceeding the limit, which in practice are
// long plain-text summaries (titles/link lines are short and tag-bearing), so
// it does not split HTML tags.
func splitLine(line string, limit int) []string {
	var out []string
	for len(line) > limit {
		cut := limit
		// back off to the last rune boundary at or before `limit`
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		// prefer a word boundary if one exists reasonably close
		if sp := strings.LastIndexByte(line[:cut], ' '); sp > limit/2 {
			cut = sp
		}
		out = append(out, strings.TrimRight(line[:cut], " "))
		line = strings.TrimLeft(line[cut:], " ")
	}
	out = append(out, line)
	return out
}
