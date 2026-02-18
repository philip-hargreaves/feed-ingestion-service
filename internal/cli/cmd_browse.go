package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	containsFilter := ""
	feedFilter := ""
	startIdx := 0

	if len(cmd.args) > 0 && !strings.HasPrefix(cmd.args[0], "-") {
		parsedLimit, err := strconv.Atoi(cmd.args[0])
		if err != nil || parsedLimit <= 0 {
			return newUsageError(fmt.Sprintf("Invalid limit %q", cmd.args[0]), "feeder browse [limit] [--contains TEXT] [--feed NAME]")
		}
		limit = parsedLimit
		startIdx = 1
	}

	for i := startIdx; i < len(cmd.args); i++ {
		arg := cmd.args[i]
		switch arg {
		case "--contains":
			if i+1 >= len(cmd.args) {
				return newUsageError("Browse requires a value after --contains", "feeder browse [limit] [--contains TEXT] [--feed NAME]")
			}
			containsFilter = cmd.args[i+1]
			i++
		case "--feed":
			if i+1 >= len(cmd.args) {
				return newUsageError("Browse requires a value after --feed", "feeder browse [limit] [--contains TEXT] [--feed NAME]")
			}
			feedFilter = cmd.args[i+1]
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return newUsageError(fmt.Sprintf("Unknown browse option %q", arg), "feeder browse [limit] [--contains TEXT] [--feed NAME]")
			}
			return newUsageError(fmt.Sprintf("Unexpected positional argument %q; only first positional limit is allowed", arg), "feeder browse [limit] [--contains TEXT] [--feed NAME]")
		}
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID:  user.ID,
		Column2: containsFilter,
		Column3: feedFilter,
		Limit:   int32(limit),
	})
	if err != nil {
		return fmt.Errorf("Couldn't get posts for user: %w", err)
	}

	fmt.Printf("Showing up to %d posts\n", limit)
	if containsFilter != "" {
		fmt.Printf("Filter contains: %s\n", containsFilter)
	}
	if feedFilter != "" {
		fmt.Printf("Filter feed: %s\n", feedFilter)
	}
	if len(posts) == 0 {
		fmt.Println("No posts found")
		return nil
	}
	fmt.Println()

	for _, post := range posts {
		fmt.Printf("Title: %s\n", post.Title)
		fmt.Printf("Feed: %s\n", post.FeedName)
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
