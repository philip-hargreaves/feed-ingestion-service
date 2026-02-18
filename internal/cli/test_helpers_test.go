package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
	"github.com/philip-hargreaves/feed-ingestion-service/internal/database"
)

func newMockState(t *testing.T, currentUser string) (*state, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed creating sqlmock: %v", err)
	}

	queries := database.New(db)
	s := &state{
		cfg: &config.Config{
			CurrentUserName: currentUser,
		},
		users:   queries,
		feeds:   queries,
		follows: queries,
		posts:   queries,
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
