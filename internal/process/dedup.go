package process

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

// SeenState maps a dedup key -> ISO date it was first delivered. Persisted to
// disk and committed back by CI so digests don't repeat across runs.
type SeenState map[string]string

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// MakeKey produces a stable dedup key. It normalizes the URL (drop scheme,
// query/fragment, lowercase host, trim trailing slash); if no usable URL, it
// falls back to a slugified title. The result is sha1-hex for compactness.
func MakeKey(a model.Article) string {
	basis := normalizeURL(a.URL)
	if basis == "" {
		basis = slugify(a.Title)
	}
	sum := sha1.Sum([]byte(basis))
	return hex.EncodeToString(sum[:])
}

func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	path := strings.TrimRight(u.Path, "/")
	return host + path
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Dedup assigns DedupKey to every article, drops cross-source duplicates
// (keeping the one with the higher ScoreSignal), and removes any key already
// present in prior. The returned slice is unique and unseen.
func Dedup(arts []model.Article, prior SeenState) []model.Article {
	best := map[string]model.Article{}
	order := []string{}

	for _, a := range arts {
		key := MakeKey(a)
		a.DedupKey = key

		if _, alreadyDelivered := prior[key]; alreadyDelivered {
			continue
		}
		if cur, ok := best[key]; ok {
			if a.ScoreSignal > cur.ScoreSignal {
				best[key] = a
			}
			continue
		}
		best[key] = a
		order = append(order, key)
	}

	out := make([]model.Article, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

// LoadSeen reads the seen-state JSON. A missing file is treated as empty state.
func LoadSeen(path string) (SeenState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SeenState{}, nil
		}
		return nil, err
	}
	state := SeenState{}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// SaveSeen records today's delivered keys into state, prunes entries older than
// 7 days, and writes the JSON back (creating parent dirs as needed).
func SaveSeen(path string, state SeenState, delivered []model.Article, now time.Time) error {
	today := now.UTC().Format("2006-01-02")
	for _, a := range delivered {
		key := a.DedupKey
		if key == "" {
			key = MakeKey(a)
		}
		state[key] = today
	}

	cutoff := now.UTC().AddDate(0, 0, -7)
	for key, iso := range state {
		t, err := time.Parse("2006-01-02", iso)
		if err != nil || t.Before(cutoff) {
			delete(state, key)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// stable key order for clean git diffs
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(keys))
	for _, k := range keys {
		ordered[k] = state[k]
	}
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
