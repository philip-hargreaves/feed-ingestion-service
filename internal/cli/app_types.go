package cli

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

type ConfigStore interface {
	SetUser(userName string) error
	GetCurrentUserName() string
}

type Store interface {
	GetUser(ctx context.Context, name string) (database.User, error)
	CreateUser(ctx context.Context, arg database.CreateUserParams) (database.User, error)
	GetUsers(ctx context.Context) ([]database.User, error)
	ResetUsers(ctx context.Context) error

	CreateFeed(ctx context.Context, arg database.CreateFeedParams) (database.Feed, error)
	GetFeeds(ctx context.Context) ([]database.GetFeedsRow, error)
	GetFeedByURL(ctx context.Context, url string) (database.Feed, error)
	MarkFeedFetched(ctx context.Context, id uuid.UUID) error
	GetNextFeedsToFetch(ctx context.Context, limit int32) ([]database.Feed, error)

	CreateFeedFollow(ctx context.Context, arg database.CreateFeedFollowParams) (database.CreateFeedFollowRow, error)
	DeleteFeedFollow(ctx context.Context, arg database.DeleteFeedFollowParams) error
	GetFeedFollowsForUser(ctx context.Context, userID uuid.UUID) ([]database.GetFeedFollowsForUserRow, error)

	CreatePost(ctx context.Context, arg database.CreatePostParams) (database.Post, error)
	GetPostsForUser(ctx context.Context, arg database.GetPostsForUserParams) ([]database.GetPostsForUserRow, error)
}

type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
}

type FeedResult struct {
	Items []FeedItem
}

type Dependencies struct {
	Config         ConfigStore
	Store          Store
	FetchFeed      func(ctx context.Context, feedURL string) (FeedResult, error)
	ExecutablePath func() (string, error)
}

type state struct {
	cfg            ConfigStore
	db             Store
	fetchFeed      func(ctx context.Context, feedURL string) (FeedResult, error)
	executablePath func() (string, error)
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

type usageError struct {
	message string
	usage   string
}

func (e usageError) Error() string {
	return fmt.Sprintf("%s\nUsage: %s", e.message, e.usage)
}

func newUsageError(message, usage string) error {
	return usageError{
		message: message,
		usage:   usage,
	}
}

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

type Command struct {
	Name string
	Args []string
}

type Registry struct {
	state *state
	cmds  *commands
}
