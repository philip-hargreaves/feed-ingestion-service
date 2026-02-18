# Feed Ingestion Service (Feeder)

Feeder is a Go CLI that ingests RSS feeds into PostgreSQL, tracks followed feeds per user, and lets you browse stored posts from the terminal.

Core features:
- User workflows: `register`, `login`, `users`, `reset`
- Feed workflows: `addfeed`, `feeds`, `follow`, `unfollow`, `following`
- Background ingestion: `agg` (concurrent workers + per-domain throttling)
- Process resilience: `supervise` (restart-on-crash with backoff)
- Post browsing: `browse` with optional filters

## Prerequisites

You need these installed to run the program:

- **Go**
- **PostgreSQL**

For local development from source, you will also use:
- `goose` for migrations
- `sqlc` when changing SQL files

## Quickstart

1. Create a PostgreSQL database (example: `ingestor`).
2. Install the CLI with `go install`.
3. Create config file at `~/.feederconfig.json`.
4. Run `register`, add feeds, then start `agg`.

Example:

```bash
# install CLI
go install github.com/philip-hargreaves/feed-ingestion-service@latest

# verify binary (name is usually the module folder name)
feed-ingestion-service help
```

Create config:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/ingestor?sslmode=disable",
  "current_user_name": ""
}
```

Run first commands:

```bash
feed-ingestion-service register alice
feed-ingestion-service addfeed "Hacker News" "https://news.ycombinator.com/rss"
feed-ingestion-service agg 5s
```

In a second terminal:

```bash
feed-ingestion-service browse 10
```

> If you prefer the command name `feeder`, build locally with `go build -o feeder .` and run `./feeder ...`.

## Install (go install)

Install the latest version:

```bash
go install github.com/philip-hargreaves/feed-ingestion-service@latest
```

Install from a specific commit/tag:

```bash
go install github.com/philip-hargreaves/feed-ingestion-service@<version-or-commit>
```

## Configuration

Feeder reads config from:
- `~/.feederconfig.json`

Required fields:
- `db_url`: Postgres connection string
- `current_user_name`: active user (managed automatically by `register`/`login`)

Example:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/ingestor?sslmode=disable",
  "current_user_name": "alice"
}
```

## Development Setup (from source)

```bash
git clone <repo-url>
cd feed-ingestion-service
```

Run migrations:

```bash
goose -dir sql/schema postgres "$DB_URL" up
```

Generate sqlc code (when SQL changes):

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

## Command Reference

### Help

```bash
feeder help
```

### User Commands

Register and set as current user:

```bash
feeder register <username>
```

Log in as existing user:

```bash
feeder login <username>
```

List users (`(Current)` marks active user):

```bash
feeder users
```

Reset all users and cascaded data:

```bash
feeder reset
```

### Feed Commands

Add feed and auto-follow it as current user:

```bash
feeder addfeed "<name>" "<url>"
```

List all feeds with owners:

```bash
feeder feeds
```

Follow existing feed by URL:

```bash
feeder follow "<url>"
```

Unfollow feed by URL:

```bash
feeder unfollow "<url>"
```

List feeds current user follows:

```bash
feeder following
```

### Aggregation Commands

Run continuous ingestion:

```bash
feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]
```

Examples:

```bash
feeder agg 10s
feeder agg 2s 4 8 1s
```

Argument defaults:
- `workers=4`
- `batch_size=workers*2`
- `domain_delay=2s` (per-domain minimum spacing)

Run ingester under supervisor (auto-restarts on crash):

```bash
feeder supervise <time_between_reqs> [workers] [batch_size] [domain_delay]
```

Example:

```bash
feeder supervise 5s 4 8 1s
```

### Browse Posts

Show posts from followed feeds:

```bash
feeder browse [limit] [--contains TEXT] [--feed NAME]
```

Examples:

```bash
feeder browse
feeder browse 10
feeder browse 10 --contains Verstappen
feeder browse 5 --contains F1 --feed RaceFans
```

## Architecture Overview

- `main.go`: app bootstrap, command registration, dispatch
- `commands.go`: command handlers, middleware, aggregation/supervisor logic
- `rss.go`: RSS fetch + XML parsing + entity handling
- `sql/schema/*`: goose migration files
- `sql/queries/*`: SQL query definitions for sqlc
- `internal/database/*`: generated typed DB layer
- `internal/config/config.go`: config file read/write

## Data Model

Core tables:
- `users`
- `feeds` (with `last_fetched_at`)
- `feed_follows` (many-to-many between users and feeds)
- `posts` (deduplicated by URL)

## Reliability Notes

- Aggregator uses worker-pool concurrency.
- Per-domain delay helps avoid over-requesting third-party hosts.
- Duplicate posts are ignored via unique URL constraint handling.
- Supervisor mode supports:
  - restart with exponential backoff
  - crash-loop protection
  - signal forwarding for graceful shutdown

## Known Behavior

- If a feed returns malformed XML, that feed is logged as an error in the scrape summary.
- `browse` shows only posts from feeds followed by the current user.

## Troubleshooting

Config not found:
- Ensure `~/.feederconfig.json` exists and is valid JSON.

Database connection errors:
- Verify `db_url`, credentials, and PostgreSQL availability.

No posts appearing:
- Ensure feeds are followed (`feeder following`) and ingester is running (`agg`/`supervise`).

