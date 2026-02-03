package cli

import (
	"bufio"
	"strings"
	"testing"
)

func TestPrompts(t *testing.T) {
	// promptString
	r := bufio.NewReader(strings.NewReader("user input\n"))
	if got := promptString(r, "test", "default"); got != "user input" {
		t.Errorf("promptString got %q, want 'user input'", got)
	}
	r = bufio.NewReader(strings.NewReader("\n"))
	if got := promptString(r, "test", "default"); got != "default" {
		t.Errorf("promptString default got %q, want 'default'", got)
	}

	// promptInt
	r = bufio.NewReader(strings.NewReader("123\n"))
	if got := promptInt(r, "test", 10); got != 123 {
		t.Errorf("promptInt got %d, want 123", got)
	}
	r = bufio.NewReader(strings.NewReader("invalid\n"))
	if got := promptInt(r, "test", 10); got != 10 {
		t.Errorf("promptInt invalid got %d, want 10", got)
	}

	// promptBool
	r = bufio.NewReader(strings.NewReader("y\n"))
	if got := promptBool(r, "test", false); got != true {
		t.Errorf("promptBool got %v, want true", got)
	}
	r = bufio.NewReader(strings.NewReader("\n"))
	if got := promptBool(r, "test", true); got != true {
		t.Errorf("promptBool default got %v, want true", got)
	}
}
