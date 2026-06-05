package deliver

import (
	"strings"
	"testing"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

func testLoc() *time.Location { return time.FixedZone("ICT", 7*60*60) }

func TestFormat_SingleMessageNoFooter(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	items := []model.DigestItem{
		{Index: 0, TitleVI: "Tin A", SummaryVI: "Tóm tắt A", Rank: 1, Category: "Sản phẩm"},
	}
	arts := []model.Article{{Source: "techcrunch", URL: "https://x.com/a"}}

	msgs := Format(items, arts, now, testLoc())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if strings.Contains(msgs[0], "Phần") {
		t.Error("single message should not carry a Phần footer")
	}
	if !strings.Contains(msgs[0], "05/06/2026") {
		t.Error("expected ICT date in header")
	}
}

func TestFormat_SplitsLongDigest(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	long := strings.Repeat("nội dung rất dài ", 60) // ~1000 chars each

	var items []model.DigestItem
	var arts []model.Article
	for i := 0; i < 12; i++ {
		items = append(items, model.DigestItem{
			Index: i, TitleVI: "Tiêu đề " + string(rune('A'+i)),
			SummaryVI: long, Rank: i + 1, Category: "Nhóm",
		})
		arts = append(arts, model.Article{Source: "src", URL: "https://x.com/p"})
	}

	msgs := Format(items, arts, now, testLoc())
	if len(msgs) < 2 {
		t.Fatalf("expected multiple messages for a long digest, got %d", len(msgs))
	}
	for i, m := range msgs {
		if len(m) > telegramMaxChars {
			t.Errorf("message %d exceeds limit: %d chars", i, len(m))
		}
		if !strings.Contains(m, "Phần") {
			t.Errorf("message %d missing Phần footer", i)
		}
		if openTags := strings.Count(m, "<b>"); openTags != strings.Count(m, "</b>") {
			t.Errorf("message %d has unbalanced <b> tags (split mid-tag?)", i)
		}
	}
}

func TestEmptyNote(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	msgs := EmptyNote(now, testLoc())
	if len(msgs) != 1 || !strings.Contains(msgs[0], "Không có tin") {
		t.Fatalf("unexpected empty note: %v", msgs)
	}
}

func TestRenderItem_EscapesHTML(t *testing.T) {
	items := []model.DigestItem{
		{Index: 0, TitleVI: "A & B <script>", SummaryVI: "x>y", Rank: 1, Category: "C"},
	}
	arts := []model.Article{{Source: "s", URL: "https://x.com/a?b=1&c=2"}}
	msgs := Format(items, arts, time.Now(), testLoc())
	joined := strings.Join(msgs, "")
	if strings.Contains(joined, "<script>") {
		t.Error("expected angle brackets escaped in title")
	}
	if !strings.Contains(joined, "&amp;") {
		t.Error("expected ampersand escaped")
	}
}
