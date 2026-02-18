package main

import (
	"context"
	"fmt"

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

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("Couldn't get current user: %w", err)
		}
		return handler(s, cmd, user)
	}
}
