// Package process turns the raw collected articles into a ranked, deduplicated
// shortlist: Filter (relevance scoring), Dedup (cross-source + persistent state),
// and Rank (composite sort + per-source caps + TopN).
package process

import (
	"strings"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

// titleWeight makes a keyword hit in the title count more than one in the body.
const (
	titleWeight = 2.0
	bodyWeight  = 1.0
)

// Filter scores each article by weighted keyword matches and keeps only those
// at or above minRelevance. The assigned score is stored on Article.Relevance.
func Filter(arts []model.Article, keywords []string, minRelevance float64) []model.Article {
	lowered := make([]string, len(keywords))
	for i, k := range keywords {
		lowered[i] = strings.ToLower(k)
	}

	var kept []model.Article
	for _, a := range arts {
		title := strings.ToLower(a.Title)
		body := strings.ToLower(a.SummaryRaw)

		var score float64
		for _, kw := range lowered {
			if kw == "" {
				continue
			}
			if strings.Contains(title, kw) {
				score += titleWeight
			}
			if strings.Contains(body, kw) {
				score += bodyWeight
			}
		}

		// arXiv items are pre-filtered by category, so keep them even with a
		// weak keyword match — the category itself is the relevance signal.
		if a.SourceType == "arxiv" && score < minRelevance {
			score = minRelevance
		}

		if score < minRelevance {
			continue
		}
		a.Relevance = score
		kept = append(kept, a)
	}
	return kept
}
