package cli

import (
	"strings"
	"testing"
)

func TestHandlerSuperviseArgsValidation(t *testing.T) {
	err := handlerSupervise(&state{}, command{name: "supervise"})
	if err == nil {
		t.Fatal("expected error when supervise is missing agg args")
	}
	if !strings.Contains(err.Error(), "Supervise requires agg arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
