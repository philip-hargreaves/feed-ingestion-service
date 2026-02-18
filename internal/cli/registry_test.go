package cli

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

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
