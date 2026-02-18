package cli

import (
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func TestHandlerBrowseRendersPosts(t *testing.T) {
	s, mock, cleanup := newMockState(t, "alice")
	defer cleanup()

	user := database.User{
		ID:   uuid.New(),
		Name: "alice",
	}
	postID := uuid.New()
	feedID := uuid.New()
	now := time.Now().UTC()

	mock.ExpectQuery(`(?s)SELECT posts.id,.*LIMIT \$4`).
		WithArgs(user.ID, "post", "sample", int32(3)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "title", "url", "description", "published_at", "feed_id", "feed_name"}).
				AddRow(postID, now, now, "Post title", "https://example.com/post", "Post description", now, feedID, "Sample Feed"),
		)

	out := captureStdout(t, func() {
		err := handlerBrowse(s, command{name: "browse", args: []string{"3", "--contains", "post", "--feed", "sample"}}, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Title: Post title") || !strings.Contains(out, "Feed: Sample Feed") {
		t.Fatalf("expected output to include post details, got: %q", out)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestHandlerBrowseLimitValidation(t *testing.T) {
	user := database.User{ID: uuid.New(), Name: "alice"}
	s := &state{}

	err := handlerBrowse(s, command{name: "browse", args: []string{"0"}}, user)
	if err == nil {
		t.Fatal("expected error for non-positive limit")
	}

	err = handlerBrowse(s, command{name: "browse", args: []string{"3", "--unknown", "x"}}, user)
	if err == nil {
		t.Fatal("expected error for unknown browse option")
	}
}
