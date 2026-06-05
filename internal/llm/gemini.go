// Package llm summarizes and translates the ranked articles into Vietnamese in
// a single structured-JSON Gemini call, with retry/backoff and a graceful
// raw-digest fallback when the API is unavailable or out of quota.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
	"github.com/qddan/ai-agent-news-bot/internal/textutil"
	"google.golang.org/genai"
)

const (
	systemInstruction = "Bạn là biên tập viên công nghệ người Việt. Nhiệm vụ: tóm tắt và dịch tin tức về AI agent sang tiếng Việt tự nhiên, súc tích, chính xác. " +
		"Với mỗi mục được đánh số, trả về title_vi (tiêu đề tiếng Việt ngắn gọn), summary_vi (1-2 câu tóm tắt tiếng Việt), " +
		"rank (độ quan trọng, 1 là quan trọng nhất), và category (một trong: \"Sản phẩm & Công cụ\", \"Nghiên cứu\", \"Cộng đồng & Thảo luận\", \"Khác\"). " +
		"Giữ nguyên thuật ngữ kỹ thuật tiếng Anh khi cần. KHÔNG bịa thông tin ngoài nội dung được cung cấp."

	maxSummaryRawChars = 400
	maxRetries         = 3
)

// Summarizer wraps a Gemini client + model id.
type Summarizer struct {
	client *genai.Client
	model  string
}

// NewSummarizer constructs a Gemini-backed summarizer.
func NewSummarizer(ctx context.Context, apiKey, modelID string) (*Summarizer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &Summarizer{client: client, model: modelID}, nil
}

// responseSchema constrains the model to emit DigestResponse-shaped JSON.
func responseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"items": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"index":      {Type: genai.TypeInteger},
						"title_vi":   {Type: genai.TypeString},
						"summary_vi": {Type: genai.TypeString},
						"rank":       {Type: genai.TypeInteger},
						"category":   {Type: genai.TypeString},
					},
					Required: []string{"index", "title_vi", "summary_vi", "rank", "category"},
				},
			},
		},
		Required: []string{"items"},
	}
}

// buildPrompt enumerates the articles so the model can reference them by index.
func buildPrompt(arts []model.Article) string {
	var b strings.Builder
	b.WriteString("Dưới đây là các tin tức cần tóm tắt và dịch (đánh số theo index):\n\n")
	for i, a := range arts {
		raw := textutil.TruncateRunes(a.SummaryRaw, maxSummaryRawChars)
		fmt.Fprintf(&b, "index=%d | nguồn=%s | tiêu đề: %s\nURL: %s\nnội dung: %s\n\n",
			i, a.Source, a.Title, a.URL, raw)
	}
	return b.String()
}

// Summarize runs one batch Gemini call. On unrecoverable failure it logs and
// returns a raw (untranslated) fallback so the pipeline can still deliver.
func (s *Summarizer) Summarize(ctx context.Context, arts []model.Article) []model.DigestItem {
	if len(arts) == 0 {
		return nil
	}

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   responseSchema(),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemInstruction}},
		},
	}
	contents := genai.Text(buildPrompt(arts))

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := s.client.Models.GenerateContent(ctx, s.model, contents, cfg)
		if err != nil {
			lastErr = err
			log.Printf("[llm] attempt %d/%d failed: %v", attempt, maxRetries, err)
			if !wait(ctx, backoff(attempt)) {
				return Fallback(arts)
			}
			continue
		}

		var parsed model.DigestResponse
		if err := json.Unmarshal([]byte(resp.Text()), &parsed); err != nil {
			lastErr = err
			log.Printf("[llm] attempt %d/%d bad JSON: %v", attempt, maxRetries, err)
			if !wait(ctx, backoff(attempt)) {
				return Fallback(arts)
			}
			continue
		}
		items := sanitize(parsed.Items, len(arts))
		if len(items) == 0 {
			lastErr = fmt.Errorf("model returned no usable items")
			if !wait(ctx, backoff(attempt)) {
				return Fallback(arts)
			}
			continue
		}
		return items
	}

	log.Printf("[llm] all attempts exhausted (%v); using raw fallback", lastErr)
	return Fallback(arts)
}

func backoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * time.Second // 1s, 4s, 9s
}

// wait sleeps for d, returning false if the context is cancelled first (caller
// should then stop retrying and fall back).
func wait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// sanitize drops items whose index is out of range and keeps category non-empty.
func sanitize(items []model.DigestItem, n int) []model.DigestItem {
	var out []model.DigestItem
	for _, it := range items {
		if it.Index < 0 || it.Index >= n {
			continue
		}
		if strings.TrimSpace(it.Category) == "" {
			it.Category = "Khác"
		}
		out = append(out, it)
	}
	return out
}

// Fallback builds an untranslated digest (original title + raw snippet) when
// the LLM is unavailable or no API key is set, so the user still gets the links.
func Fallback(arts []model.Article) []model.DigestItem {
	items := make([]model.DigestItem, 0, len(arts))
	for i, a := range arts {
		summary := strings.TrimSpace(a.SummaryRaw)
		if len([]rune(summary)) > 200 {
			summary = textutil.TruncateRunes(summary, 200) + "…"
		}
		items = append(items, model.DigestItem{
			Index:     i,
			TitleVI:   a.Title,
			SummaryVI: summary,
			Rank:      i + 1,
			Category:  "Khác (chưa dịch — LLM tạm gián đoạn)",
		})
	}
	return items
}
