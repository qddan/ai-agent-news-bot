package process

import (
	"math"
	"sort"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

// Caps bounds how many items a single source may contribute, so noisy sources
// (arXiv, Reddit) can't crowd out the digest.
type Caps struct {
	Arxiv  int
	Reddit int
}

// Rank sorts articles by a composite score (relevance + popularity + recency),
// applies per-source caps, and returns at most topN items.
func Rank(arts []model.Article, caps Caps, topN int, now time.Time) []model.Article {
	scored := make([]model.Article, len(arts))
	copy(scored, arts)

	sort.SliceStable(scored, func(i, j int) bool {
		return compositeScore(scored[i], now) > compositeScore(scored[j], now)
	})

	var (
		out         []model.Article
		arxivCount  int
		redditCount int
	)
	for _, a := range scored {
		switch a.SourceType {
		case "arxiv":
			if caps.Arxiv > 0 && arxivCount >= caps.Arxiv {
				continue
			}
			arxivCount++
		case "reddit":
			if caps.Reddit > 0 && redditCount >= caps.Reddit {
				continue
			}
			redditCount++
		}
		out = append(out, a)
		if topN > 0 && len(out) >= topN {
			break
		}
	}
	return out
}

// compositeScore blends relevance, a dampened popularity signal, and recency.
func compositeScore(a model.Article, now time.Time) float64 {
	relevance := a.Relevance
	popularity := math.Log1p(a.ScoreSignal) // dampen large HN/Reddit counts

	ageHours := now.Sub(a.Published).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	recency := math.Max(0, 1.0-ageHours/48.0) // linear decay over 48h

	return relevance*1.5 + popularity*1.0 + recency*1.0
}
