package cli

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func handlerAddFeed(s *state, cmd command, user database.User) error {
	ctx := commandContext(cmd)
	if len(cmd.args) != 2 {
		return newUsageError("Addfeed requires two arguments: name and url", "feeder addfeed <name> <url>")
	}

	now := time.Now()
	feed, err := s.feeds.CreateFeed(ctx, database.CreateFeedParams{
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

	_, err = s.follows.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
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
	ctx := commandContext(cmd)
	if len(cmd.args) != 1 {
		return newUsageError("Follow requires one argument: url", "feeder follow <url>")
	}

	feed, err := s.feeds.GetFeedByURL(ctx, cmd.args[0])
	if err != nil {
		return fmt.Errorf("Couldn't find feed by URL: %w", err)
	}

	now := time.Now()
	feedFollow, err := s.follows.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
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
	ctx := commandContext(cmd)
	if len(cmd.args) != 1 {
		return newUsageError("Unfollow requires one argument: url", "feeder unfollow <url>")
	}

	feed, err := s.feeds.GetFeedByURL(ctx, cmd.args[0])
	if err != nil {
		return fmt.Errorf("Couldn't find feed by URL: %w", err)
	}

	err = s.follows.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{
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
	ctx := commandContext(cmd)
	if len(cmd.args) > 0 {
		return newUsageError("Following command does not take any arguments", "feeder following")
	}

	feedFollows, err := s.follows.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("Couldn't get feed follows: %w", err)
	}

	for _, feedFollow := range feedFollows {
		fmt.Println(feedFollow.FeedName)
	}

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	ctx := commandContext(cmd)
	if len(cmd.args) > 0 {
		return newUsageError("Feeds command does not take any arguments", "feeder feeds")
	}

	feeds, err := s.feeds.GetFeeds(ctx)
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
