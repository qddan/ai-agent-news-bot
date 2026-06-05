package process

import (
	"testing"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

func TestFilter_TitleWeightedAndThreshold(t *testing.T) {
	kw := []string{"AI agent", "agentic"}
	arts := []model.Article{
		{Title: "New AI agent framework", SummaryRaw: "an agentic system"}, // title+body hits
		{Title: "Unrelated cooking blog", SummaryRaw: "recipes"},           // no hit -> dropped
		{Title: "weather", SummaryRaw: "AI agent mentioned in passing"},    // body-only hit
	}
	out := Filter(arts, kw, 1.0)
	if len(out) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(out))
	}
	if out[0].Relevance <= out[1].Relevance {
		t.Errorf("expected title+body match to outscore body-only: %v vs %v",
			out[0].Relevance, out[1].Relevance)
	}
}

func TestFilter_ArxivKeptDespiteWeakMatch(t *testing.T) {
	arts := []model.Article{
		{SourceType: "arxiv", Title: "Some math paper", SummaryRaw: "proofs"},
	}
	out := Filter(arts, []string{"AI agent"}, 1.0)
	if len(out) != 1 {
		t.Fatalf("expected arxiv item kept by category signal, got %d", len(out))
	}
}
