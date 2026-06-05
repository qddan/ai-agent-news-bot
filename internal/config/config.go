// Package config centralizes all tunable pipeline parameters and secret loading.
// Secrets come only from the environment (.env is loaded for local runs);
// nothing sensitive is ever hard-coded here.
package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every knob the pipeline reads. Tune sources/thresholds here.
type Config struct {
	// Secrets (from env / .env)
	TelegramBotToken string
	TelegramChatID   string
	GeminiAPIKey     string

	// LLM
	GeminiModel string

	// Sources
	Keywords   []string
	RSSFeeds   []string
	ArxivCats  []string
	RedditSubs []string

	// Ranking / windowing
	TopN          int
	ArxivCap      int
	RedditCap     int
	WindowHours   int
	MinRelevance  float64
	SeenStatePath string

	// Locale
	Location *time.Location
}

// Load builds a Config from defaults + environment. It best-effort loads a
// local .env file (ignored if absent, e.g. in CI where secrets are real env vars).
func Load() *Config {
	_ = godotenv.Load() // no-op if .env missing; CI injects real env vars

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*60*60) // fallback if tzdata unavailable
	}

	return &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),

		GeminiModel: "gemini-2.5-flash",

		Keywords: []string{
			"AI agent", "agentic", "LLM agent", "autonomous agent",
			"multi-agent", "agent framework", "tool use", "function calling",
			"agent workflow", "ReAct", "RAG agent", "MCP", "agent orchestration",
		},
		RSSFeeds: []string{
			"https://techcrunch.com/category/artificial-intelligence/feed/",
			"https://venturebeat.com/category/ai/feed/",
			"https://www.theverge.com/rss/ai-artificial-intelligence/index.xml",
			"https://www.technologyreview.com/topic/artificial-intelligence/feed",
		},
		ArxivCats:  []string{"cs.AI", "cs.MA", "cs.CL"},
		RedditSubs: []string{"AI_Agents", "LocalLLaMA", "MachineLearning"},

		TopN:          15,
		ArxivCap:      4,
		RedditCap:     5,
		WindowHours:   36,
		MinRelevance:  1.0,
		SeenStatePath: "state/seen.json",

		Location: loc,
	}
}
