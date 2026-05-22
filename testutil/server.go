// Package testutil provides shared helpers for tests across the project. In
// Phase 0 it contains only the minimum needed to prove the new structure is
// importable and that servers can be started in isolation. It will grow in
// later phases (mock RESP client, RESP assertions, fake clock, timeout harness,
// leak checks) — but we add helpers as real tests need them, not speculatively.
package testutil

import (
	"net"
	"testing"

	"github.com/codecrafters-io/redis-starter-go/internal/server"
)

// StartServer boots a Server bound to an OS-assigned free port on loopback and
// returns its address. It registers cleanup via t.Cleanup, so callers do not
// have to remember to shut it down. Because each call constructs a fresh
// Server (own store, own queue, own consumer), multiple servers can run in the
// same test binary without interfering — the property the old global queue
// denied us.
func StartServer(t *testing.T) string {
	t.Helper()

	s := server.New()
	addr, err := s.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}

	go s.Serve()
	t.Cleanup(func() { _ = s.Close() })

	return addr.String()
}

// Dial opens a TCP connection to addr and registers cleanup. It is the seed of
// the mock RESP client that Phase 4 will flesh out.
func Dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
