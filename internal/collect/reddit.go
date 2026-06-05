// Package collect — Reddit source.
//
// Reddit's /new.json is aggressively blocked from datacenter IPs (CI runners,
// cloud egress) with HTTP 403 regardless of User-Agent. The .rss endpoint at
// /new/.rss is served by a separate, more permissive layer and works where
// /new.json does not. We use it as the primary fetch path; ups count is lost
// but Title/URL/Published are sufficient for downstream ranking.
package collect

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
)

type redditCollector struct {
	subs   []string
	parser *gofeed.Parser
}

const redditUserAgent = "ai-agent-news-bot/1.0 (by /u/qddan)"

// NewReddit builds the Reddit collector over the given subreddits.
func NewReddit(subs []string) Collector {
	p := gofeed.NewParser()
	p.UserAgent = redditUserAgent
	return &redditCollector{subs: subs, parser: p}
}

func (r *redditCollector) Name() string { return "reddit" }

func (r *redditCollector) Collect(ctx context.Context, since time.Time) ([]model.Article, error) {
	var out []model.Article

	for _, sub := range r.subs {
		endpoint := fmt.Sprintf("https://www.reddit.com/r/%s/new/.rss?limit=50", sub)
		feed, err := r.parser.ParseURLWithContext(endpoint, ctx)
		if err != nil {
			log.Printf("[collect] reddit r/%s failed: %v", sub, err)
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
				Source:     "reddit:r/" + sub,
				SourceType: "reddit",
				Title:      textutil.Plain(it.Title),
				URL:        it.Link,
				SummaryRaw: textutil.Plain(it.Description),
				Published:  pub.UTC(),
			})
		}
	}
	return out, nil
}
