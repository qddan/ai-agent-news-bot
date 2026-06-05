package process

import (
	"testing"
	"time"

	"github.com/qddan/ai-agent-news-bot/internal/model"
)

func TestMakeKey_NormalizesURL(t *testing.T) {
	a := model.Article{URL: "https://www.Example.com/path/?utm_source=x#frag"}
	b := model.Article{URL: "http://example.com/path"}
	if MakeKey(a) != MakeKey(b) {
		t.Fatalf("expected normalized URLs to share a key:\n a=%s\n b=%s", MakeKey(a), MakeKey(b))
	}
}

func TestMakeKey_FallsBackToTitle(t *testing.T) {
	a := model.Article{Title: "Hello World"}
	if MakeKey(a) == "" {
		t.Fatal("expected non-empty key from title slug")
	}
	b := model.Article{Title: "hello   world"}
	if MakeKey(a) != MakeKey(b) {
		t.Fatal("expected slugified titles to match")
	}
}

func TestDedup_DropsSeenAndKeepsHigherScore(t *testing.T) {
	arts := []model.Article{
		{URL: "https://a.com/x", ScoreSignal: 5},
		{URL: "https://a.com/x", ScoreSignal: 50}, // dup, higher score wins
		{URL: "https://b.com/y"},                  // already seen -> dropped
	}
	prior := SeenState{MakeKey(model.Article{URL: "https://b.com/y"}): "2026-06-01"}

	out := Dedup(arts, prior)
	if len(out) != 1 {
		t.Fatalf("expected 1 unique unseen article, got %d", len(out))
	}
	if out[0].ScoreSignal != 50 {
		t.Fatalf("expected higher-score duplicate kept, got score %v", out[0].ScoreSignal)
	}
}

func TestSaveLoadSeen_PrunesOld(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/seen.json"
	now := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)

	state := SeenState{
		"old": "2026-05-01", // >7d old -> pruned
	}
	delivered := []model.Article{{URL: "https://new.com/z", DedupKey: "newkey"}}
	if err := SaveSeen(path, state, delivered, now); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSeen(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["old"]; ok {
		t.Error("expected old entry pruned")
	}
	if _, ok := loaded["newkey"]; !ok {
		t.Error("expected delivered key persisted")
	}
}

func TestLoadSeen_MissingFileIsEmpty(t *testing.T) {
	s, err := LoadSeen(t.TempDir() + "/does-not-exist.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 0 {
		t.Fatalf("expected empty state, got %d", len(s))
	}
}
