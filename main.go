package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/cli"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		fmt.Printf("error reading config: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		fmt.Printf("error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	dbQueries := database.New(db)
	registry := cli.NewRegistry(cli.Dependencies{
		Config: &cfg,
		Users:  dbQueries,
		Feeds:  dbQueries,
		Follows: dbQueries,
		Posts:  dbQueries,
		FetchFeed: func(ctx context.Context, feedURL string) (cli.FeedResult, error) {
			feed, err := fetchFeed(ctx, feedURL)
			if err != nil {
				return cli.FeedResult{}, err
			}
			items := make([]cli.FeedItem, 0, len(feed.Channel.Item))
			for _, item := range feed.Channel.Item {
				items = append(items, cli.FeedItem{
					Title:       item.Title,
					Link:        item.Link,
					Description: item.Description,
					PubDate:     item.PubDate,
				})
			}
			return cli.FeedResult{Items: items}, nil
		},
	})

	if len(os.Args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	cmdName := os.Args[1]
	cmdArgs := os.Args[2:]

	cmd := cli.Command{
		Name: cmdName,
		Args: cmdArgs,
	}

	err = registry.Run(context.Background(), cmd)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
