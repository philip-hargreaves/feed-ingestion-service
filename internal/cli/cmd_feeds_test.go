package cli

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestHandlerFeedsPrintsNameURLUser(t *testing.T) {
	s, mock, cleanup := newMockState(t, "alice")
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT feeds.name, feeds.url, users.name AS user_name FROM feeds INNER JOIN users ON feeds.user_id = users.id ORDER BY feeds.created_at ASC")).
		WillReturnRows(
			sqlmock.NewRows([]string{"name", "url", "user_name"}).
				AddRow("F1 Feed", "https://example.com/rss", "alice"),
		)

	out := captureStdout(t, func() {
		if err := handlerFeeds(s, command{name: "feeds"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Name: F1 Feed") ||
		!strings.Contains(out, "URL: https://example.com/rss") ||
		!strings.Contains(out, "User: alice") {
		t.Fatalf("expected feed output with name/url/user, got: %q", out)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestHandlerFeedsArgsValidation(t *testing.T) {
	err := handlerFeeds(&state{}, command{name: "feeds", args: []string{"extra"}})
	if err == nil {
		t.Fatal("expected usage error when feeds has args")
	}
}
