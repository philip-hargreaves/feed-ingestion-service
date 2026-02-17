package main

import (
	"errors"
	"fmt"

	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
)

type state struct {
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("login requires a username argument")
	}

	username := cmd.args[0]
	err := s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("couldn't set user: %w", err)
	}

	fmt.Printf("User has been set to %q\n", username)
	return nil
}
