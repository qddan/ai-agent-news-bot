package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
)

// hnCollector queries the HN Algolia search-by-date API for each keyword.
type hnCollector struct {
	keywords []string
	client   *http.Client
}

// NewHackerNews builds the Hacker News collector.
func NewHackerNews(keywords []string) Collector {
	return &hnCollector{keywords: keywords, client: &http.Client{Timeout: 20 * time.Second}}
}

func (h *hnCollector) Name() string { return "hackernews" }

type hnResponse struct {
	Hits []struct {
		ObjectID  string `json:"objectID"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		Points    int    `json:"points"`
		CreatedAt int64  `json:"created_at_i"`
		StoryText string `json:"story_text"`
	} `json:"hits"`
}

func (h *hnCollector) Collect(ctx context.Context, since time.Time) ([]model.Article, error) {
	seen := map[string]bool{}
	var out []model.Article

	for _, kw := range h.keywords {
		q := url.Values{}
		q.Set("query", kw)
		q.Set("tags", "story")
		q.Set("numericFilters", fmt.Sprintf("created_at_i>%d", since.Unix()))
		q.Set("hitsPerPage", "50")
		endpoint := "https://hn.algolia.com/api/v1/search_by_date?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := h.client.Do(req)
		if err != nil {
			log.Printf("[collect] hn query %q failed: %v", kw, err)
			continue
		}
		var data hnResponse
		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(&data)
		resp.Body.Close()
		if decErr != nil {
			log.Printf("[collect] hn query %q decode failed: %v", kw, decErr)
			continue
		}

		for _, hit := range data.Hits {
			if seen[hit.ObjectID] {
				continue // same story matched by multiple keywords
			}
			seen[hit.ObjectID] = true

			link := hit.URL
			if link == "" { // Ask HN / text posts -> use the HN permalink
				link = "https://news.ycombinator.com/item?id=" + hit.ObjectID
			}
			out = append(out, model.Article{
				Source:      "hackernews",
				SourceType:  "hn",
				Title:       hit.Title,
				URL:         link,
				SummaryRaw:  textutil.Plain(hit.StoryText),
				Published:   time.Unix(hit.CreatedAt, 0).UTC(),
				ScoreSignal: float64(hit.Points),
			})
		}
	}
	return out, nil
}
