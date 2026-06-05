// Package collect fetches raw articles from every configured source.
// Each source is a Collector; RunAll fans them out concurrently and a failure
// in one source never aborts the whole run (logged and skipped).
package collect

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/config"
	"github.com/qddan/ai-agent-news-bot/internal/model"
	"golang.org/x/sync/errgroup"
)

// Collector fetches articles published at or after `since`.
type Collector interface {
	Name() string
	Collect(ctx context.Context, since time.Time) ([]model.Article, error)
}

// Build assembles the collector set from config.
func Build(cfg *config.Config) []Collector {
	return []Collector{
		NewRSS(cfg.RSSFeeds),
		NewHackerNews(cfg.Keywords),
		NewArxiv(cfg.ArxivCats),
		NewReddit(cfg.RedditSubs),
	}
}

// RunAll runs every collector concurrently. One source erroring out is logged
// and dropped; the run continues with whatever the others returned.
func RunAll(ctx context.Context, collectors []Collector, since time.Time) []model.Article {
	var (
		mu  sync.Mutex
		all []model.Article
	)
	g, ctx := errgroup.WithContext(ctx)

	for _, c := range collectors {
		c := c
		g.Go(func() error {
			arts, err := c.Collect(ctx, since)
			if err != nil {
				log.Printf("[collect] %s failed: %v", c.Name(), err)
				return nil // never fail-fast — partial results are fine
			}
			log.Printf("[collect] %s -> %d articles", c.Name(), len(arts))
			mu.Lock()
			all = append(all, arts...)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait() // we swallowed all errors above; Wait only joins goroutines
	return all
}
