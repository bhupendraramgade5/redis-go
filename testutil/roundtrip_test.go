package testutil_test

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/testutil"
)

// TestSetGet_OverTCP proves the entire restructured project wires together: a
// Server is constructed and started on a random port, a client connects over
// real TCP, and a SET/GET round-trip returns the expected RESP bytes. This is
// the black-box counterpart to the command package's white-box test.
//
// It also implicitly proves the de-globalization worked: StartServer can be
// called repeatedly (here, and in any other test) without servers sharing a
// queue or store.
func TestSetGet_OverTCP(t *testing.T) {
	addr := testutil.StartServer(t)
	conn := testutil.Dial(t, addr)

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	r := bufio.NewReader(conn)

	// SET k hello
	if _, err := conn.Write([]byte(encodeArray("SET", "k", "hello"))); err != nil {
		t.Fatalf("write SET: %v", err)
	}
	if got := readLine(t, r); got != "+OK" {
		t.Fatalf("SET reply = %q, want %q", got, "+OK")
	}

	// GET k  ->  $5\r\nhello\r\n
	if _, err := conn.Write([]byte(encodeArray("GET", "k"))); err != nil {
		t.Fatalf("write GET: %v", err)
	}
	header := readLine(t, r)
	if header != "$5" {
		t.Fatalf("GET header = %q, want %q", header, "$5")
	}
	body := readLine(t, r)
	if body != "hello" {
		t.Fatalf("GET body = %q, want %q", body, "hello")
	}
}

// TestTwoServers_AreIsolated is the payoff test for Phase 0: two servers in one
// process, each with its own store. A key set on one must not appear on the
// other. Under the old package-global queue this could not even be expressed.
func TestTwoServers_AreIsolated(t *testing.T) {
	addrA := testutil.StartServer(t)
	addrB := testutil.StartServer(t)

	connA := testutil.Dial(t, addrA)
	connB := testutil.Dial(t, addrB)
	_ = connA.SetDeadline(time.Now().Add(2 * time.Second))
	_ = connB.SetDeadline(time.Now().Add(2 * time.Second))
	rA := bufio.NewReader(connA)
	rB := bufio.NewReader(connB)

	// Set key only on server A.
	connA.Write([]byte(encodeArray("SET", "only-on-a", "1")))
	if got := readLine(t, rA); got != "+OK" {
		t.Fatalf("server A SET reply = %q", got)
	}

	// Server B must not have it.
	connB.Write([]byte(encodeArray("GET", "only-on-a")))
	if got := readLine(t, rB); got != "$-1" {
		t.Fatalf("server B sees key from server A (got %q) — servers are NOT isolated", got)
	}
}

// encodeArray builds a RESP array command from parts.
func encodeArray(parts ...string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%d\r\n", len(parts)))
	for _, p := range parts {
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(p), p))
	}
	return b.String()
}

// readLine reads one CRLF-terminated line and returns it without the trailing
// \r\n.
func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}
