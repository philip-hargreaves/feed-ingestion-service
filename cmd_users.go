package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return newUsageError("Login requires a username argument", "feeder login <username>")
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
		return newUsageError("Register requires a username argument", "feeder register <username>")
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
		return newUsageError("Users command does not take any arguments", "feeder users")
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

func handlerReset(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return newUsageError("Reset command does not take any arguments", "feeder reset")
	}

	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Couldn't reset database: %w", err)
	}

	fmt.Println("Database reset successfully")
	return nil
}
