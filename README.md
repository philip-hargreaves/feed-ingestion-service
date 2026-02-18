# Feeder

Feeder is a Go CLI for building a personal or team "news inbox" from RSS feeds.  
It pulls feeds continuously, stores normalised data in PostgreSQL, and lets you query posts from the terminal.

## Why Use Feeder

If you follow many engineering or industry feeds, reading directly from websites is noisy and fragmented.
Feeder gives you:

- A single searchable stream from multiple RSS sources
- Persistent history in your own database
- Multi-user feed-follow model (useful for local teams/dev groups)
- Reliable ingestion loop with process supervision and restart backoff
- Fast terminal-native browsing for low-friction daily use


## Core Features

- **User management**: `register`, `login`, `users`, `reset`
- **Feed management**: `addfeed`, `feeds`, `follow`, `unfollow`, `following`
- **Continuous ingestion**: `agg` (aggregate) with worker concurrency, batching, and per-domain throttling
- **Resilience**: `supervise` restarts `agg` with exponential backoff and crash-loop protection
- **Post discovery**: `browse` with default 10 posts, plus `--contains` and `--feed` filters

## Prerequisites

Required:
- **Go**
- **PostgreSQL**

For development from source:
- `goose` (migrations)
- `sqlc` (regenerating typed DB code when SQL changes)

## Quickstart

1. Create a PostgreSQL database (example: `ingestor`).
2. Install the CLI with `go install`.
3. Create `~/.feederconfig.json`.
4. Register a user, add a feed, and start the aggregator.

```bash
# install latest
go install github.com/philip-hargreaves/feed-ingestion-service@latest

# run help (binary name is usually module folder name)
feed-ingestion-service help
```

Create config file:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/ingestor?sslmode=disable",
  "current_user_name": ""
}
```

Run:

```bash
feed-ingestion-service register alice
feed-ingestion-service addfeed "Hacker News" "https://news.ycombinator.com/rss"
feed-ingestion-service agg 5s
```

In another terminal:

```bash
feed-ingestion-service browse
```


## Installation



```bash
go install github.com/philip-hargreaves/feed-ingestion-service@latest
```


## Configuration

Feeder reads config from:
- `~/.feederconfig.json`

Fields:
- `db_url`: PostgreSQL connection string
- `current_user_name`: current active user (managed by `register`/`login`)

Example:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/ingestor?sslmode=disable",
  "current_user_name": "alice"
}
```

## Command Reference

### User Commands

```bash
feeder register <username>
feeder login <username>
feeder users
feeder reset
```

### Feed Commands

```bash
feeder addfeed "<name>" "<url>"
feeder feeds
feeder follow "<url>"
feeder unfollow "<url>"
feeder following
```

### Aggregation Commands

```bash
feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]
feeder supervise <time_between_reqs> [workers] [batch_size] [domain_delay]
```

Defaults:
- `workers=4`
- `batch_size=workers*2`
- `domain_delay=2s`

### Browse

```bash
feeder browse [limit] [--contains TEXT] [--feed NAME]
```

Examples:

```bash
feeder browse
feeder browse 5
feeder browse 5 --contains F1
feeder browse 5 --contains Verstappen --feed RaceFans
```

## Architecture

- `main.go`: bootstrap and dependency wiring
- `internal/cli/*`: command registry, handlers, middleware, CLI tests
- `rss.go`: RSS fetch + XML parsing + HTML entity handling
- `internal/config/config.go`: local config read/write
- `sql/schema/*`: goose migrations
- `sql/queries/*`: SQL definitions for sqlc
- `internal/database/*`: generated typed query layer

## Data Model

- `users`
- `feeds` (includes `last_fetched_at`)
- `feed_follows` (many-to-many users <-> feeds)
- `posts` (unique by URL to prevent duplicates)

## Reliability Notes

- Worker-pool ingestion for throughput
- Per-domain request delay to avoid hammering hosts
- Duplicate post inserts safely ignored
- Supervisor mode with:
  - exponential backoff
  - crash-loop protection
  - signal forwarding for graceful shutdown
- RSS HTTP client timeout to avoid hanging requests

## Development

```bash
git clone <repo-url>
cd feed-ingestion-service
```

Migrations:

```bash
goose -dir sql/schema postgres "$DB_URL" up
```

Regenerate SQLC code (after SQL edits):

```bash
sqlc generate
```

Run tests:

```bash
go test ./...
```

Run from source:

```bash
go run . help
```

## Troubleshooting

Config not found:
- Ensure `~/.feederconfig.json` exists and is valid JSON.

DB connection issues:
- Verify `db_url`, credentials, host, and PostgreSQL status.

No posts showing:
- Confirm the user follows feeds (`feeder following`) and ingestion is running (`agg` or `supervise`).


