package cli

import "fmt"

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

  browse [limit] [--contains TEXT] [--feed NAME]
    Show posts for followed feeds with optional filtering (default limit: 10).
`

func handlerHelp(_ *state, cmd command) error {
	if len(cmd.args) > 0 {
		return newUsageError("Help command does not take any arguments", "feeder help")
	}

	fmt.Print(helpText)
	return nil
}
