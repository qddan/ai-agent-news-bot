// Command digest runs the AI-agent news pipeline once: collect → filter →
// dedup → rank → summarize (Vietnamese) → deliver to Telegram.
//
// Flags:
//
//	--once          run the pipeline a single time (default behavior)
//	--dry-run       build the digest but print it instead of sending; skip state writes
//	--get-chat-id   call Telegram getUpdates and print discovered chat IDs, then exit
//	--notify-errors on failure, send a short error note to the Telegram chat
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/collect"
	"github.com/qddan/ai-agent-news-bot/internal/config"
	"github.com/qddan/ai-agent-news-bot/internal/deliver"
	"github.com/qddan/ai-agent-news-bot/internal/llm"
	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/process"
)

func main() {
	var (
		_         = flag.Bool("once", true, "run the pipeline a single time")
		dryRun    = flag.Bool("dry-run", false, "print the digest instead of sending; skip state writes")
		getChatID = flag.Bool("get-chat-id", false, "print Telegram chat IDs from getUpdates and exit")
		notifyErr = flag.Bool("notify-errors", false, "send a Telegram note on failure")
	)
	flag.Parse()

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if *getChatID {
		runGetChatID(ctx, cfg)
		return
	}

	if err := run(ctx, cfg, *dryRun); err != nil {
		log.Printf("[digest] FAILED: %v", err)
		if *notifyErr && cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
			deliver.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID).NotifyError(ctx, err.Error())
		}
		os.Exit(1)
	}
}

func runGetChatID(ctx context.Context, cfg *config.Config) {
	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required for --get-chat-id")
	}
	tg := deliver.NewTelegram(cfg.TelegramBotToken, "")
	id, err := tg.GetChatID(ctx)
	if err != nil {
		log.Fatalf("get-chat-id: %v", err)
	}
	fmt.Printf("\nTELEGRAM_CHAT_ID=%s\n", id)
}

func run(ctx context.Context, cfg *config.Config, dryRun bool) error {
	if !dryRun {
		if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
			return fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required (use --dry-run to skip sending)")
		}
	}

	now := time.Now()
	since := now.Add(-time.Duration(cfg.WindowHours) * time.Hour)

	// 1. Collect (concurrent, fault-tolerant)
	collectors := collect.Build(cfg)
	raw := collect.RunAll(ctx, collectors, since)
	log.Printf("[digest] collected %d raw articles", len(raw))

	// 2. Filter by keyword relevance
	filtered := process.Filter(raw, cfg.Keywords, cfg.MinRelevance)
	log.Printf("[digest] %d after relevance filter", len(filtered))

	// 3. Dedup against persistent seen-state + cross-source
	seen, err := process.LoadSeen(cfg.SeenStatePath)
	if err != nil {
		return fmt.Errorf("load seen state: %w", err)
	}
	deduped := process.Dedup(filtered, seen)
	log.Printf("[digest] %d after dedup", len(deduped))

	// 4. Rank + per-source caps + TopN
	ranked := process.Rank(deduped, process.Caps{Arxiv: cfg.ArxivCap, Reddit: cfg.RedditCap}, cfg.TopN, now)
	log.Printf("[digest] %d after rank/cap (TopN=%d)", len(ranked), cfg.TopN)

	// Empty digest -> send a short note so we know the job ran.
	if len(ranked) == 0 {
		messages := deliver.EmptyNote(now, cfg.Location)
		return finishEmpty(ctx, cfg, messages, dryRun)
	}

	// 5. Summarize + translate to Vietnamese (with raw fallback)
	items := summarize(ctx, cfg, ranked)

	// 6. Format into Telegram-sized HTML messages
	messages := deliver.Format(items, ranked, now, cfg.Location)
	log.Printf("[digest] formatted into %d message(s)", len(messages))

	if dryRun {
		printDryRun(messages)
		return nil
	}

	// 7. Deliver
	tg := deliver.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	if err := tg.SendAll(ctx, messages); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	// 8. Persist seen-state only after a successful send
	if err := process.SaveSeen(cfg.SeenStatePath, seen, ranked, now); err != nil {
		return fmt.Errorf("save seen state: %w", err)
	}
	log.Printf("[digest] done — sent %d message(s), state updated", len(messages))
	return nil
}

// summarize builds the Summarizer and runs it. If the client can't be
// constructed (e.g. missing key in dry-run), it returns the raw fallback.
func summarize(ctx context.Context, cfg *config.Config, ranked []model.Article) []model.DigestItem {
	if cfg.GeminiAPIKey == "" {
		log.Printf("[digest] no GEMINI_API_KEY — using raw fallback digest")
		return llm.Fallback(ranked)
	}
	s, err := llm.NewSummarizer(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Printf("[digest] gemini client init failed (%v) — using raw fallback", err)
		return llm.Fallback(ranked)
	}
	return s.Summarize(ctx, ranked)
}

// finishEmpty delivers (or prints) the no-news note without touching seen-state.
func finishEmpty(ctx context.Context, cfg *config.Config, messages []string, dryRun bool) error {
	if dryRun {
		printDryRun(messages)
		return nil
	}
	tg := deliver.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	if err := tg.SendAll(ctx, messages); err != nil {
		return fmt.Errorf("send empty note: %w", err)
	}
	log.Printf("[digest] no news today — sent note")
	return nil
}

// printDryRun dumps the rendered messages to stdout for local inspection.
func printDryRun(messages []string) {
	fmt.Printf("\n===== DRY RUN: %d message(s) =====\n", len(messages))
	for i, m := range messages {
		fmt.Printf("\n----- message %d/%d (%d chars) -----\n%s\n", i+1, len(messages), len(m), m)
	}
}
