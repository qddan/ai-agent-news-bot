package collect

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
)

// arxivCollector queries the arXiv Atom API per category. arXiv asks clients to
// sleep ~3s between requests, which we honor.
type arxivCollector struct {
	categories []string
	parser     *gofeed.Parser
}

// NewArxiv builds the arXiv collector over the given categories (e.g. cs.AI).
func NewArxiv(categories []string) Collector {
	return &arxivCollector{categories: categories, parser: gofeed.NewParser()}
}

func (a *arxivCollector) Name() string { return "arxiv" }

const arxivThrottle = 3 * time.Second

func (a *arxivCollector) Collect(ctx context.Context, since time.Time) ([]model.Article, error) {
	var out []model.Article

	for i, cat := range a.categories {
		if i > 0 {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(arxivThrottle):
			}
		}

		q := url.Values{}
		q.Set("search_query", "cat:"+cat)
		q.Set("sortBy", "submittedDate")
		q.Set("sortOrder", "descending")
		q.Set("max_results", "50")
		// arXiv 301-redirects http -> https and gofeed won't follow it, so use https directly.
		endpoint := "https://export.arxiv.org/api/query?" + q.Encode()

		feed, err := a.fetchWithRetry(ctx, endpoint, cat)
		if err != nil {
			log.Printf("[collect] arxiv cat %s failed: %v", cat, err)
			continue
		}
		for _, it := range feed.Items {
			pub := itemTime(it)
			if pub.Before(since) {
				continue
			}
			if it.Link == "" {
				continue
			}
			out = append(out, model.Article{
				Source:     fmt.Sprintf("arxiv:%s", cat),
				SourceType: "arxiv",
				Title:      textutil.Plain(it.Title),
				URL:        it.Link,
				SummaryRaw: textutil.Plain(it.Description),
				Published:  pub.UTC(),
			})
		}
	}
	return out, nil
}

// fetchWithRetry calls arXiv with exponential backoff specifically on 429s.
// arXiv's published rate-limit advice is "1 request per 3 seconds"; the shared
// export.arxiv.org host enforces this aggressively when multiple clients share
// the same egress IP (CI runners). Other errors fail fast — only rate-limit
// recovery benefits from retrying.
func (a *arxivCollector) fetchWithRetry(ctx context.Context, endpoint, cat string) (*gofeed.Feed, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * 10 * time.Second // 10s, 20s
			log.Printf("[collect] arxiv cat %s 429 — backing off %s (attempt %d/%d)", cat, backoff, attempt+1, maxAttempts)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		feed, err := a.parser.ParseURLWithContext(endpoint, ctx)
		if err == nil {
			return feed, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "429") {
			return nil, err
		}
	}
	return nil, lastErr
}
