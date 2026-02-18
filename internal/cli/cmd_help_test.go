package cli

import (
	"strings"
	"testing"
)

func TestHandlerHelpNoArgsPrintsHelp(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handlerHelp(&state{}, command{name: "help"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Feeder CLI") {
		t.Fatalf("expected help output to contain title, got: %q", out)
	}
}

func TestHandlerHelpArgsError(t *testing.T) {
	err := handlerHelp(&state{}, command{name: "help", args: []string{"extra"}})
	if err == nil {
		t.Fatal("expected error when help is called with arguments")
	}
}
