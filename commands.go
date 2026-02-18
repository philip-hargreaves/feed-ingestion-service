package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

type state struct {
	cfg *config.Config
	db  *database.Queries
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

const helpText = `Feeder CLI

Feeder is an RSS aggregation CLI that lets you:
- Manage users
- Add and follow feeds
- Aggregate posts from followed feeds
- Browse saved posts

Usage:
  feeder <command> [arguments]

Commands:
  help
    Show this help message.

  register <username>
    Create a user and set it as the current user.

  login <username>
    Set an existing user as the current user.

  users
    List all users and mark the current user.

  reset
    Delete all users (and cascaded data such as feeds, follows, and posts).

  addfeed <name> <url>
    Create a feed for the current user and auto-follow it.

  feeds
    List all feeds with feed URL and owner.

  follow <url>
    Follow an existing feed by URL.

  unfollow <url>
    Unfollow a feed by URL.

  following
    List feed names the current user follows.

  agg <time_between_reqs> [workers] [batch_size] [domain_delay]
    Continuously fetch feeds on an interval.
    Optional args default to workers=4, batch_size=workers*2, domain_delay=2s.

  supervise <agg-args...>
    Run agg under a supervisor that restarts on crashes with exponential backoff.

  browse [limit]
    Show recent posts for followed feeds. Default limit is 2.
`

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("Unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("Couldn't get current user: %w", err)
		}
		return handler(s, cmd, user)
	}
}

func handlerHelp(_ *state, cmd command) error {
	if len(cmd.args) > 0 {
		return errors.New("Help command does not take any arguments")
	}

	fmt.Print(helpText)
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Login requires a username argument")
	}

	username := cmd.args[0]

	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("User %q not found", username)
	}

	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("Couldn't set user: %w", err)
	}

	fmt.Printf("User has been set to %q\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Register requires a username argument")
	}

	username := cmd.args[0]
	now := time.Now()

	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      username,
	})
	if err != nil {
		return fmt.Errorf("Couldn't create user: %w", err)
	}

	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("Couldn't set user: %w", err)
	}

	fmt.Printf("User %q was created\n", user.Name)
	return nil
}

func handlerUsers(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return errors.New("Users command does not take any arguments")
	}

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Couldn't get users: %w", err)
	}

	for _, user := range users {
		if user.Name == s.cfg.CurrentUserName {
			fmt.Printf("* %s (Current)\n", user.Name)
			continue
		}
		fmt.Printf("* %s\n", user.Name)
	}

	return nil
}

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
	feedName   string
	newPosts   int
	duplicates int
	err        error
}

func scrapeFeed(s *state, nextFeed database.Feed, limiter *domainRateLimiter) scrapeResult {
	result := scrapeResult{
		feedName: nextFeed.Name,
	}

	fmt.Printf("Fetching feed: %s\n", nextFeed.Name)
	err := s.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		result.err = fmt.Errorf("Couldn't mark feed fetched for %q: %w", nextFeed.Name, err)
		return result
	}

	limiter.wait(nextFeed.Url)
	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		result.err = fmt.Errorf("Couldn't fetch feed %q: %w", nextFeed.Name, err)
		return result
	}

	for _, item := range feed.Channel.Item {
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
	for result := range results {
		totalNewPosts += result.newPosts
		totalDuplicates += result.duplicates
		if result.err != nil {
			totalErrors++
			fmt.Printf("Error scraping feed %q: %v\n", result.feedName, result.err)
		}
	}

	fmt.Printf(
		"Scrape cycle complete (feeds=%d, workers=%d, new_posts=%d, duplicates=%d, errors=%d)\n",
		len(nextFeeds),
		workers,
		totalNewPosts,
		totalDuplicates,
		totalErrors,
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
		return errors.New("Agg requires 1 to 4 arguments: time_between_reqs [workers] [batch_size] [domain_delay]")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("Invalid duration %q: %w", cmd.args[0], err)
	}

	fmt.Printf("Collecting feeds every %s\n", timeBetweenRequests)

	workers := 4
	if len(cmd.args) >= 2 {
		workers, err = strconv.Atoi(cmd.args[1])
		if err != nil || workers < 1 {
			return fmt.Errorf("Invalid workers value %q", cmd.args[1])
		}
	}

	batchSize := workers * 2
	if len(cmd.args) >= 3 {
		batchSize, err = strconv.Atoi(cmd.args[2])
		if err != nil || batchSize < 1 {
			return fmt.Errorf("Invalid batch size value %q", cmd.args[2])
		}
	}

	domainDelay := 2 * time.Second
	if len(cmd.args) == 4 {
		domainDelay, err = time.ParseDuration(cmd.args[3])
		if err != nil || domainDelay <= 0 {
			return fmt.Errorf("Invalid domain delay %q", cmd.args[3])
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

func handlerSupervise(_ *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("Supervise requires agg arguments, for example: supervise 10s 4 8 2s")
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Couldn't determine executable path: %w", err)
	}

	const (
		maxRestartsInWindow = 5
		restartWindow       = time.Minute
		baseBackoff         = time.Second
		maxBackoff          = 30 * time.Second
	)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	var restartTimes []time.Time
	restartAttempt := 0

	for {
		childArgs := append([]string{"agg"}, cmd.args...)
		childCmd := exec.Command(executablePath, childArgs...)
		childCmd.Stdout = os.Stdout
		childCmd.Stderr = os.Stderr
		childCmd.Stdin = os.Stdin

		err := childCmd.Start()
		if err != nil {
			return fmt.Errorf("Couldn't start agg child process: %w", err)
		}

		fmt.Printf("Supervisor started agg (pid=%d)\n", childCmd.Process.Pid)

		childDone := make(chan error, 1)
		go func() {
			childDone <- childCmd.Wait()
		}()

		startedAt := time.Now()

		select {
		case receivedSignal := <-signalChan:
			fmt.Printf("Supervisor received signal %s. Stopping agg.\n", receivedSignal.String())
			_ = childCmd.Process.Signal(receivedSignal)

			select {
			case <-childDone:
			case <-time.After(10 * time.Second):
				fmt.Println("Agg did not stop gracefully, killing process.")
				_ = childCmd.Process.Kill()
				<-childDone
			}
			return nil

		case waitErr := <-childDone:
			if waitErr == nil {
				fmt.Println("Agg exited cleanly. Supervisor stopping.")
				return nil
			}

			now := time.Now()
			restartTimes = append(restartTimes, now)
			filtered := restartTimes[:0]
			for _, t := range restartTimes {
				if now.Sub(t) <= restartWindow {
					filtered = append(filtered, t)
				}
			}
			restartTimes = filtered

			if len(restartTimes) > maxRestartsInWindow {
				return fmt.Errorf("Agg crashed too often (%d times in %s), supervisor stopping", len(restartTimes), restartWindow)
			}

			if time.Since(startedAt) > restartWindow {
				restartAttempt = 0
			}

			backoff := baseBackoff * time.Duration(1<<restartAttempt)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			if restartAttempt < 30 {
				restartAttempt++
			}

			fmt.Printf("Agg crashed: %v\n", waitErr)
			fmt.Printf("Restarting agg in %s...\n", backoff)

			timer := time.NewTimer(backoff)
			select {
			case receivedSignal := <-signalChan:
				timer.Stop()
				fmt.Printf("Supervisor received signal %s during backoff. Exiting.\n", receivedSignal.String())
				return nil
			case <-timer.C:
			}
		}
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 2 {
		return errors.New("Addfeed requires two arguments: name and url")
	}

	now := time.Now()
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("Couldn't create feed: %w", err)
	}

	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Couldn't create feed follow: %w", err)
	}

	fmt.Printf("%+v\n", feed)
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return errors.New("Follow requires one argument: url")
	}

	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("Couldn't find feed by URL: %w", err)
	}

	now := time.Now()
	feedFollow, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Couldn't create feed follow: %w", err)
	}

	fmt.Printf("Feed Name: %s\n", feedFollow.FeedName)
	fmt.Printf("User Name: %s\n", feedFollow.UserName)
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return errors.New("Unfollow requires one argument: url")
	}

	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("Couldn't find feed by URL: %w", err)
	}

	err = s.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Couldn't unfollow feed: %w", err)
	}

	fmt.Printf("Unfollowed feed: %s\n", feed.Name)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.args) > 0 {
		return errors.New("Following command does not take any arguments")
	}

	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Couldn't get feed follows: %w", err)
	}

	for _, feedFollow := range feedFollows {
		fmt.Println(feedFollow.FeedName)
	}

	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) > 1 {
		return errors.New("Browse accepts at most one optional argument: limit")
	}
	if len(cmd.args) == 1 {
		parsedLimit, err := strconv.Atoi(cmd.args[0])
		if err != nil || parsedLimit <= 0 {
			return fmt.Errorf("Invalid limit %q", cmd.args[0])
		}
		limit = parsedLimit
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("Couldn't get posts for user: %w", err)
	}

	for _, post := range posts {
		fmt.Printf("Title: %s\n", post.Title)
		fmt.Printf("URL: %s\n", post.Url)
		if post.Description.Valid {
			fmt.Printf("Description: %s\n", post.Description.String)
		}
		if post.PublishedAt.Valid {
			fmt.Printf("Published At: %s\n", post.PublishedAt.Time.Format(time.RFC3339))
		}
		fmt.Println()
	}

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return errors.New("Feeds command does not take any arguments")
	}

	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Couldn't get feeds: %w", err)
	}

	for _, feed := range feeds {
		fmt.Printf("Name: %s\n", feed.Name)
		fmt.Printf("URL: %s\n", feed.Url)
		fmt.Printf("User: %s\n\n", feed.UserName)
	}

	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return errors.New("Reset command does not take any arguments")
	}

	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Couldn't reset database: %w", err)
	}

	fmt.Println("Database reset successfully")
	return nil
}
