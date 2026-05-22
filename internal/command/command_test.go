package command

import (
	"testing"

	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

// TestSetGet_RoundTrip is the Phase 0 "does the new structure work" test at the
// unit level. It is white-box (same package as the code under test), so it can
// construct a store directly and call Execute. Each test owns its own store —
// no shared global state — which is exactly the isolation Phase 0 unlocked.
func TestSetGet_RoundTrip(t *testing.T) {
	s := store.New()
	set := SetCommand{Store: s}
	get := GetCommand{Store: s}

	if got := set.Execute([]string{"SET", "k", "hello"}); got != "+OK\r\n" {
		t.Fatalf("SET reply = %q, want %q", got, "+OK\r\n")
	}

	want := "$5\r\nhello\r\n"
	if got := get.Execute([]string{"GET", "k"}); got != want {
		t.Fatalf("GET reply = %q, want %q", got, want)
	}

	// Missing key returns null bulk string.
	if got := get.Execute([]string{"GET", "missing"}); got != "$-1\r\n" {
		t.Fatalf("GET missing = %q, want %q", got, "$-1\r\n")
	}
}
