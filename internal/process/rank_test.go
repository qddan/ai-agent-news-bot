package process

import (
	"testing"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

func TestRank_AppliesCapsAndTopN(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	var arts []model.Article
	for i := 0; i < 10; i++ {
		arts = append(arts, model.Article{SourceType: "arxiv", Relevance: 5, Published: now})
		arts = append(arts, model.Article{SourceType: "reddit", Relevance: 5, Published: now})
	}
	for i := 0; i < 10; i++ {
		arts = append(arts, model.Article{SourceType: "rss", Relevance: 5, Published: now})
	}

	out := Rank(arts, Caps{Arxiv: 4, Reddit: 5}, 15, now)
	if len(out) != 15 {
		t.Fatalf("expected TopN=15, got %d", len(out))
	}
	var arxiv, reddit int
	for _, a := range out {
		switch a.SourceType {
		case "arxiv":
			arxiv++
		case "reddit":
			reddit++
		}
	}
	if arxiv > 4 {
		t.Errorf("arxiv cap exceeded: %d", arxiv)
	}
	if reddit > 5 {
		t.Errorf("reddit cap exceeded: %d", reddit)
	}
}

func TestRank_HigherRelevanceSortsFirst(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	arts := []model.Article{
		{SourceType: "rss", Relevance: 1, Published: now},
		{SourceType: "rss", Relevance: 9, Published: now},
	}
	out := Rank(arts, Caps{}, 10, now)
	if out[0].Relevance != 9 {
		t.Fatalf("expected highest relevance first, got %v", out[0].Relevance)
	}
}
