package collect

import (
	"context"
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
)

// rssCollector pulls items from a list of RSS/Atom feed URLs via gofeed.
type rssCollector struct {
	feeds  []string
	parser *gofeed.Parser
}

// NewRSS builds the RSS collector over the given feed URLs.
func NewRSS(feeds []string) Collector {
	return &rssCollector{feeds: feeds, parser: gofeed.NewParser()}
}

func (r *rssCollector) Name() string { return "rss" }

func (r *rssCollector) Collect(ctx context.Context, since time.Time) ([]model.Article, error) {
	var out []model.Article
	for _, url := range r.feeds {
		feed, err := r.parser.ParseURLWithContext(url, ctx)
		if err != nil {
			// one bad feed shouldn't sink the rest
			log.Printf("[collect] rss feed %s failed: %v", url, err)
			continue
		}
		label := feed.Title
		if label == "" {
			label = url
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
				Source:     label,
				SourceType: "rss",
				Title:      textutil.Plain(it.Title),
				URL:        it.Link,
				SummaryRaw: textutil.Plain(it.Description),
				Published:  pub.UTC(),
			})
		}
	}
	return out, nil
}

// itemTime returns the published/updated time in UTC, falling back to now.
func itemTime(it *gofeed.Item) time.Time {
	if it.PublishedParsed != nil {
		return *it.PublishedParsed
	}
	if it.UpdatedParsed != nil {
		return *it.UpdatedParsed
	}
	return time.Now()
}
