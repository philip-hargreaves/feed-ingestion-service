package cli

import (
	"strings"
	"testing"
)

func TestParsePublishedAt(t *testing.T) {
	valid := parsePublishedAt("Mon, 06 Sep 2021 12:00:00 GMT")
	if !valid.Valid {
		t.Fatal("expected RFC1123 timestamp to parse")
	}

	invalid := parsePublishedAt("not-a-date")
	if invalid.Valid {
		t.Fatal("expected invalid timestamp to return null time")
	}
}

func TestHandlerAggArgumentValidation(t *testing.T) {
	s := &state{}

	err := handlerAgg(s, command{name: "agg"})
	if err == nil {
		t.Fatal("expected error when agg is missing duration")
	}
	if !strings.Contains(err.Error(), "Agg requires 1 to 4 arguments") {
		t.Fatalf("unexpected error: %v", err)
	}

	err = handlerAgg(s, command{name: "agg", args: []string{"not-duration"}})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
