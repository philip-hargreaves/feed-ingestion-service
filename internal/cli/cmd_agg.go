package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

type domainRateLimiter struct {
	mu          sync.Mutex
	nextAllowed map[string]time.Time
	interval    time.Duration
}

func newDomainRateLimiter(interval time.Duration) *domainRateLimiter {
	return &domainRateLimiter{
		nextAllowed: make(map[string]time.Time),
		interval:    interval,
	}
}

func (l *domainRateLimiter) wait(rawURL string) {
	parsed, err := url.Parse(rawURL)
	host := rawURL
	if err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}

	for {
		l.mu.Lock()
		now := time.Now()
		next := l.nextAllowed[host]
		if !now.Before(next) {
			l.nextAllowed[host] = now.Add(l.interval)
			l.mu.Unlock()
			return
		}
		waitDuration := next.Sub(now)
		l.mu.Unlock()
		time.Sleep(waitDuration)
	}
}

type scrapeResult struct {
	feedName    string
	newPosts    int
	duplicates  int
	parseErrors int
	err         error
}

func scrapeFeed(s *state, nextFeed database.Feed, limiter *domainRateLimiter) scrapeResult {
	result := scrapeResult{
		feedName: nextFeed.Name,
	}

	fmt.Printf("Fetching feed: %s\n", nextFeed.Name)
	limiter.wait(nextFeed.Url)
	feed, err := s.fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		if strings.Contains(err.Error(), "Couldn't parse feed XML") {
			result.parseErrors = 1
		}
		result.err = fmt.Errorf("Couldn't fetch feed %q: %w", nextFeed.Name, err)
		return result
	}

	err = s.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		result.err = fmt.Errorf("Couldn't mark feed fetched for %q: %w", nextFeed.Name, err)
		return result
	}

	for _, item := range feed.Items {
		_, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Title:     item.Title,
			Url:       item.Link,
			Description: sql.NullString{
				String: item.Description,
				Valid:  strings.TrimSpace(item.Description) != "",
			},
			PublishedAt: parsePublishedAt(item.PubDate),
			FeedID:      nextFeed.ID,
		})
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				result.duplicates++
				continue
			}
			fmt.Printf("Error creating post %q: %v\n", item.Title, err)
			continue
		}
		result.newPosts++
		fmt.Printf("Saved post: %s\n", item.Title)
	}

	return result
}

func scrapeFeeds(s *state, workers int, batchSize int, limiter *domainRateLimiter) error {
	nextFeeds, err := s.db.GetNextFeedsToFetch(context.Background(), int32(batchSize))
	if err != nil {
		return fmt.Errorf("Couldn't get next feeds to fetch: %w", err)
	}
	if len(nextFeeds) == 0 {
		fmt.Println("No feeds to fetch")
		return nil
	}

	if workers < 1 {
		workers = 1
	}
	if workers > len(nextFeeds) {
		workers = len(nextFeeds)
	}

	jobs := make(chan database.Feed, len(nextFeeds))
	results := make(chan scrapeResult, len(nextFeeds))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for feed := range jobs {
				results <- scrapeFeed(s, feed, limiter)
			}
		}()
	}

	for _, feed := range nextFeeds {
		jobs <- feed
	}
	close(jobs)

	wg.Wait()
	close(results)

	totalNewPosts := 0
	totalDuplicates := 0
	totalErrors := 0
	totalParseErrors := 0
	for result := range results {
		totalNewPosts += result.newPosts
		totalDuplicates += result.duplicates
		totalParseErrors += result.parseErrors
		if result.err != nil {
			totalErrors++
			fmt.Printf("Error scraping feed %q: %v\n", result.feedName, result.err)
		}
	}

	fmt.Printf(
		"Scrape cycle complete (feeds=%d, workers=%d, new_posts=%d, duplicates=%d, errors=%d, parse_errors=%d)\n",
		len(nextFeeds),
		workers,
		totalNewPosts,
		totalDuplicates,
		totalErrors,
		totalParseErrors,
	)
	return nil
}

func parsePublishedAt(pubDate string) sql.NullTime {
	trimmed := strings.TrimSpace(pubDate)
	if trimmed == "" {
		return sql.NullTime{}
	}

	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04 MST",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return sql.NullTime{
				Time:  parsed,
				Valid: true,
			}
		}
	}

	return sql.NullTime{}
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 || len(cmd.args) > 4 {
		return newUsageError("Agg requires 1 to 4 arguments", "feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return newUsageError(fmt.Sprintf("Invalid duration %q", cmd.args[0]), "feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]")
	}

	fmt.Printf("Collecting feeds every %s\n", timeBetweenRequests)

	workers := 4
	if len(cmd.args) >= 2 {
		workers, err = strconv.Atoi(cmd.args[1])
		if err != nil || workers < 1 {
			return newUsageError(fmt.Sprintf("Invalid workers value %q", cmd.args[1]), "feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]")
		}
	}

	batchSize := workers * 2
	if len(cmd.args) >= 3 {
		batchSize, err = strconv.Atoi(cmd.args[2])
		if err != nil || batchSize < 1 {
			return newUsageError(fmt.Sprintf("Invalid batch size value %q", cmd.args[2]), "feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]")
		}
	}

	domainDelay := 2 * time.Second
	if len(cmd.args) == 4 {
		domainDelay, err = time.ParseDuration(cmd.args[3])
		if err != nil || domainDelay <= 0 {
			return newUsageError(fmt.Sprintf("Invalid domain delay %q", cmd.args[3]), "feeder agg <time_between_reqs> [workers] [batch_size] [domain_delay]")
		}
	}

	fmt.Printf("Agg config: workers=%d batch_size=%d domain_delay=%s\n", workers, batchSize, domainDelay)

	limiter := newDomainRateLimiter(domainDelay)

	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		err := scrapeFeeds(s, workers, batchSize, limiter)
		if err != nil {
			fmt.Printf("Error scraping feeds: %v\n", err)
		}
	}
}
