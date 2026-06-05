# AI-Agent News Digest Bot → Telegram

Mỗi sáng **09:00 ICT**, bot thu thập tin tức về *AI agent* từ nhiều nguồn, lọc/tóm tắt/dịch sang **tiếng Việt** bằng Gemini, và gửi 1 bản digest gọn về **Telegram**.

Chạy hoàn toàn miễn phí trên **GitHub Actions** (cron), không phụ thuộc máy cá nhân.

## Nguồn tin

- **RSS/Atom** — TechCrunch AI, VentureBeat AI, The Verge AI, MIT Tech Review
- **Hacker News** — qua Algolia search-by-date theo keyword
- **arXiv** — categories `cs.AI`, `cs.MA`, `cs.CL`
- **Reddit** — `r/AI_Agents`, `r/LocalLLaMA`, `r/MachineLearning`

## Pipeline

```
collect (song song, lỗi 1 nguồn không chết cả run)
  → filter (chấm điểm relevance theo keyword)
  → dedup (chuẩn hóa URL/slug + seen.json, cửa sổ 36h)
  → rank (relevance + popularity + recency, cap mỗi nguồn, TopN=15)
  → summarize (Gemini, JSON có cấu trúc, dịch tiếng Việt; fallback digest thô khi hết quota)
  → format (HTML group theo category, chia ≤4096 char/message)
  → deliver (Telegram sendMessage, retry honor retry_after)
  → cập nhật seen.json (commit ngược repo qua workflow)
```

## Cấu trúc

```
cmd/digest/main.go        entrypoint + flags
internal/config           tham số pipeline + load secrets từ env/.env
internal/model            Article + DigestItem/DigestResponse
internal/collect          rss / hackernews / arxiv / reddit + RunAll (errgroup)
internal/process          filter / dedup / rank
internal/llm              Gemini summarize (structured JSON + fallback)
internal/deliver          format (HTML + split) / telegram (send + get-chat-id)
state/seen.json           dedup state (commit ngược qua CI)
.github/workflows         daily-digest.yml (cron 02:00 UTC)
```

## Chạy local

Yêu cầu **Go ≥ 1.23**.

```bash
cp .env.example .env          # điền TELEGRAM_BOT_TOKEN, GEMINI_API_KEY
go mod tidy

# Lấy TELEGRAM_CHAT_ID: nhắn 1 tin cho bot trước, rồi:
make get-chat-id              # in chat_id → thêm vào .env

make dry-run                  # build digest, in ra màn hình, KHÔNG gửi, KHÔNG ghi state
make run                      # gửi thật về Telegram
```

### Flags

| Flag | Ý nghĩa |
|---|---|
| `--once` | chạy pipeline 1 lần (mặc định) |
| `--dry-run` | build digest, in ra thay vì gửi; bỏ qua ghi state |
| `--get-chat-id` | gọi getUpdates, in các chat_id, rồi thoát |
| `--notify-errors` | gửi note lỗi ngắn về Telegram khi fail |

## Deploy (GitHub Actions)

1. Push repo: `git push -u origin main`
2. Set 3 secrets ở **Settings → Secrets and variables → Actions**:
   `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `GEMINI_API_KEY`
3. **Actions → Daily AI-Agent Digest → Run workflow** để test (dispatch).
4. Cron `0 2 * * *` (02:00 UTC = 09:00 ICT) tự chạy mỗi ngày.

> ⚠️ **Bảo mật:** không bao giờ commit `.env`. Token + key chỉ ở `.env` (gitignored) và GitHub Actions secrets.

## Tinh chỉnh

Mọi tham số ở `internal/config/config.go`: keywords, danh sách nguồn, `TopN`, cap mỗi nguồn, `WindowHours`, model Gemini (`gemini-2.5-flash`).
