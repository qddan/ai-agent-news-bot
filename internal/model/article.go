// Package model holds the shared data contract passed between pipeline stages:
// collectors produce []Article, the LLM returns DigestResponse.
package model

import "time"

// Article is the common schema every collector normalizes its source into.
type Article struct {
	Source      string    // human label, e.g. "techcrunch" | "hackernews" | "arxiv" | "reddit:r/AI_Agents"
	SourceType  string    // "rss" | "hn" | "arxiv" | "reddit"
	Title       string    // original title (usually EN)
	URL         string    // canonical link (kept in original language)
	SummaryRaw  string    // snippet / abstract / selftext (may be empty)
	Published   time.Time // normalized to UTC
	ScoreSignal float64   // HN points / reddit upvotes (0 for rss/arxiv)
	Relevance   float64   // keyword relevance score assigned by process.Filter
	DedupKey    string    // stable key assigned by process.Dedup
}

// DigestItem is one entry returned by the LLM (mapped from ResponseSchema).
type DigestItem struct {
	Index     int    `json:"index"`      // back-reference into the ranked []Article
	TitleVI   string `json:"title_vi"`   // Vietnamese title
	SummaryVI string `json:"summary_vi"` // Vietnamese summary
	Rank      int    `json:"rank"`       // editorial importance (1 = most important)
	Category  string `json:"category"`   // grouping label, e.g. "Sản phẩm", "Nghiên cứu"
}

// DigestResponse is the top-level JSON object the LLM is constrained to emit.
type DigestResponse struct {
	Items []DigestItem `json:"items"`
}
