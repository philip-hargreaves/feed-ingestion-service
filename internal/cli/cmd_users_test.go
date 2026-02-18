package cli

import (
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestHandlerUsersPrintsCurrentUserMarker(t *testing.T) {
	s, mock, cleanup := newMockState(t, "alice")
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, created_at, updated_at, name FROM users ORDER BY created_at ASC")).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "name"}).
				AddRow(uuid.New(), now, now, "alice").
				AddRow(uuid.New(), now, now, "bob"),
		)

	out := captureStdout(t, func() {
		if err := handlerUsers(s, command{name: "users"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "* alice (Current)") || !strings.Contains(out, "* bob") {
		t.Fatalf("expected users output with current marker, got: %q", out)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestHandlerUsersArgsValidation(t *testing.T) {
	err := handlerUsers(&state{}, command{name: "users", args: []string{"extra"}})
	if err == nil {
		t.Fatal("expected usage error when users has args")
	}
}
