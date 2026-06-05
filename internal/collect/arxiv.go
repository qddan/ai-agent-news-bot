package collect

import (
	"context"
	"fmt"
	"log"
	"net/url"
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

		feed, err := a.parser.ParseURLWithContext(endpoint, ctx)
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
