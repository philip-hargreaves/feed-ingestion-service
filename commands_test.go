package main

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func newMockState(t *testing.T, currentUser string) (*state, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed creating sqlmock: %v", err)
	}

	s := &state{
		cfg: &config.Config{
			CurrentUserName: currentUser,
		},
		db: database.New(db),
	}

	cleanup := func() {
		_ = db.Close()
	}

	return s, mock, cleanup
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String()
}

func TestCommandsRunUnknownCommand(t *testing.T) {
	c := &commands{
		handlers: map[string]func(*state, command) error{},
	}

	err := c.run(&state{}, command{name: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "Unknown command") {
		t.Fatalf("expected unknown command error, got: %v", err)
	}
}

func TestCommandsRunCallsRegisteredHandler(t *testing.T) {
	called := false
	c := &commands{
		handlers: map[string]func(*state, command) error{},
	}
	c.register("ping", func(_ *state, cmd command) error {
		called = true
		if cmd.name != "ping" {
			t.Fatalf("unexpected command passed to handler: %s", cmd.name)
		}
		return nil
	})

	if err := c.run(&state{}, command{name: "ping"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected registered handler to be called")
	}
}

func TestMiddlewareLoggedInSuccess(t *testing.T) {
	s, mock, cleanup := newMockState(t, "alice")
	defer cleanup()

	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, created_at, updated_at, name FROM users WHERE NAME = $1")).
		WithArgs("alice").
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "name"}).
				AddRow(userID, now, now, "alice"),
		)

	called := false
	handler := middlewareLoggedIn(func(_ *state, _ command, user database.User) error {
		called = true
		if user.ID != userID || user.Name != "alice" {
			t.Fatalf("unexpected user in middleware: %+v", user)
		}
		return nil
	})

	if err := handler(s, command{name: "needs-login"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestMiddlewareLoggedInUserLookupError(t *testing.T) {
	s, mock, cleanup := newMockState(t, "missing-user")
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, created_at, updated_at, name FROM users WHERE NAME = $1")).
		WithArgs("missing-user").
		WillReturnError(sql.ErrNoRows)

	handler := middlewareLoggedIn(func(_ *state, _ command, _ database.User) error {
		return errors.New("should not be called")
	})

	err := handler(s, command{name: "needs-login"})
	if err == nil {
		t.Fatal("expected error from middleware")
	}
	if !strings.Contains(err.Error(), "Couldn't get current user") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestHandlerHelpNoArgsPrintsHelp(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handlerHelp(&state{}, command{name: "help"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Feeder CLI") {
		t.Fatalf("expected help output to contain title, got: %q", out)
	}
	if !strings.Contains(out, "browse [limit]") {
		t.Fatalf("expected help output to contain browse command, got: %q", out)
	}
}

func TestHandlerHelpArgsError(t *testing.T) {
	err := handlerHelp(&state{}, command{name: "help", args: []string{"extra"}})
	if err == nil {
		t.Fatal("expected error when help is called with arguments")
	}
}

func TestParsePublishedAt(t *testing.T) {
	valid := parsePublishedAt("Mon, 06 Sep 2021 12:00:00 GMT")
	if !valid.Valid {
		t.Fatal("expected RFC1123 timestamp to parse")
	}

	invalid := parsePublishedAt("not-a-date")
	if invalid.Valid {
		t.Fatal("expected invalid timestamp to return null time")
	}

	empty := parsePublishedAt("   ")
	if empty.Valid {
		t.Fatal("expected empty timestamp to return null time")
	}
}

func TestHandlerAggArgumentValidation(t *testing.T) {
	err := handlerAgg(&state{}, command{name: "agg"})
	if err == nil {
		t.Fatal("expected error when agg is missing duration")
	}
	if !strings.Contains(err.Error(), "Agg requires one argument") {
		t.Fatalf("unexpected error: %v", err)
	}

	err = handlerAgg(&state{}, command{name: "agg", args: []string{"not-duration"}})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "Invalid duration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandlerBrowseDefaultLimitAndOutput(t *testing.T) {
	s, mock, cleanup := newMockState(t, "alice")
	defer cleanup()

	user := database.User{
		ID:   uuid.New(),
		Name: "alice",
	}
	postID := uuid.New()
	feedID := uuid.New()
	now := time.Now().UTC()

	mock.ExpectQuery(`(?s)SELECT posts.id,.*LIMIT \$2`).
		WithArgs(user.ID, int32(2)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "title", "url", "description", "published_at", "feed_id"}).
				AddRow(postID, now, now, "Post title", "https://example.com/post", "Post description", now, feedID),
		)

	out := captureStdout(t, func() {
		if err := handlerBrowse(s, command{name: "browse"}, user); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Title: Post title") {
		t.Fatalf("expected output to include post title, got: %q", out)
	}
	if !strings.Contains(out, "URL: https://example.com/post") {
		t.Fatalf("expected output to include post url, got: %q", out)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestHandlerBrowseLimitValidation(t *testing.T) {
	user := database.User{ID: uuid.New(), Name: "alice"}
	s := &state{}

	err := handlerBrowse(s, command{name: "browse", args: []string{"1", "2"}}, user)
	if err == nil {
		t.Fatal("expected error when too many args are provided")
	}

	err = handlerBrowse(s, command{name: "browse", args: []string{"0"}}, user)
	if err == nil {
		t.Fatal("expected error for non-positive limit")
	}

	err = handlerBrowse(s, command{name: "browse", args: []string{"abc"}}, user)
	if err == nil {
		t.Fatal("expected error for non-numeric limit")
	}
}
