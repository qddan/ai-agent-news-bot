package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
)

// redditCollector reads each subreddit's public /new.json listing. Reddit
// throttles default Go/library user-agents with 429s, so a custom UA is required.
type redditCollector struct {
	subs   []string
	client *http.Client
}

// NewReddit builds the Reddit collector over the given subreddits.
func NewReddit(subs []string) Collector {
	return &redditCollector{subs: subs, client: &http.Client{Timeout: 20 * time.Second}}
}

func (r *redditCollector) Name() string { return "reddit" }

const redditUserAgent = "ai-agent-news-bot/1.0 (by /u/qddan)"

type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				Title      string  `json:"title"`
				URL        string  `json:"url"`
				Permalink  string  `json:"permalink"`
				Selftext   string  `json:"selftext"`
				Ups        float64 `json:"ups"`
				CreatedUTC float64 `json:"created_utc"`
				Subreddit  string  `json:"subreddit"`
				IsSelf     bool    `json:"is_self"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func (r *redditCollector) Collect(ctx context.Context, since time.Time) ([]model.Article, error) {
	var out []model.Article

	for _, sub := range r.subs {
		endpoint := fmt.Sprintf("https://www.reddit.com/r/%s/new.json?limit=50", sub)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", redditUserAgent)

		resp, err := r.client.Do(req)
		if err != nil {
			log.Printf("[collect] reddit r/%s request failed: %v", sub, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("[collect] reddit r/%s status %d (Reddit often blocks datacenter IPs)", sub, resp.StatusCode)
			resp.Body.Close()
			continue
		}
		var listing redditListing
		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(&listing)
		resp.Body.Close()
		if decErr != nil {
			log.Printf("[collect] reddit r/%s decode failed: %v", sub, decErr)
			continue
		}

		for _, child := range listing.Data.Children {
			d := child.Data
			pub := time.Unix(int64(d.CreatedUTC), 0).UTC()
			if pub.Before(since) {
				continue
			}
			// For self/text posts prefer the discussion permalink; else the linked URL.
			link := d.URL
			if d.IsSelf || link == "" {
				link = "https://www.reddit.com" + d.Permalink
			}
			if link == "" {
				continue
			}
			out = append(out, model.Article{
				Source:      "reddit:r/" + d.Subreddit,
				SourceType:  "reddit",
				Title:       d.Title,
				URL:         link,
				SummaryRaw:  textutil.Plain(d.Selftext),
				Published:   pub,
				ScoreSignal: d.Ups,
			})
		}
	}
	return out, nil
}
