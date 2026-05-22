package command

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

// Client holds the per-connection transaction state: an optional in-progress
// MULTI (tx) and the set of WATCHed keys with the versions observed at WATCH
// time.
//
// Phase 0 boundary note: in the original code Client also carried a net.Conn.
// That field is gone here — the connection lives in the server layer, which is
// the only place that reads from / writes to the socket. handleCommand never
// touched the conn, so removing it costs nothing and keeps the command package
// free of any networking dependency. This is the kind of cleanup the package
// split *reveals*: a field that was incidentally co-located with logic that
// did not need it.
type Client struct {
	Store   *store.DataStore
	tx      *TxContext
	watched map[string]int64
}

// NewClient creates per-connection command state bound to a store.
func NewClient(s *store.DataStore) *Client {
	return &Client{Store: s}
}

// TxContext is the queue of commands accumulated between MULTI and EXEC.
type TxContext struct {
	commandqueue [][]string
}

// Handle is the top-level command dispatcher (formerly handleCommand). It
// implements the transaction state machine (MULTI/EXEC/DISCARD/WATCH/UNWATCH)
// and otherwise either queues a command (inside MULTI) or executes it directly.
//
// WARNING (left intact for Phase 3): EXEC takes the store lock and then replays
// queued commands via ExecuteDirect, several of which take the same
// non-reentrant lock — a self-deadlock. Likewise an infinite BLPOP/XREAD blocks
// whichever goroutine runs it. These are real bugs and are deliberately NOT
// fixed in Phase 0; the roadmap pins them with failing tests in Phase 3 before
// any fix.
func Handle(client *Client, registry map[string]Command, args []string) string {
	if len(args) == 0 {
		return "-ERR unknown command\r\n"
	}

	cmd := strings.ToUpper(args[0])

	switch cmd {
	case "MULTI":
		if client.tx != nil {
			return "-ERR MULTI calls can not be nested\r\n"
		}
		client.tx = &TxContext{}
		return "+OK\r\n"

	case "EXEC":
		if client.tx == nil {
			return "-ERR EXEC without MULTI\r\n"
		}

		client.Store.Lock()
		defer client.Store.Unlock()

		for key, version := range client.watched {
			if client.Store.KeyVersion[key] != version {
				client.tx = nil
				client.watched = nil
				return "*-1\r\n" // abort transaction
			}
		}

		var responses []string
		for _, queued := range client.tx.commandqueue {
			responses = append(responses, ExecuteDirect(registry, queued))
		}
		client.watched = nil
		client.tx = nil

		return encodeEXECArray(responses)

	case "DISCARD":
		if client.tx == nil {
			return "-ERR DISCARD without MULTI\r\n"
		}
		client.watched = nil
		client.tx = nil
		return "+OK\r\n"

	case "WATCH":
		client.Store.Lock()
		defer client.Store.Unlock()

		if client.tx != nil {
			return "-ERR WATCH inside MULTI is not allowed\r\n"
		}

		if client.watched == nil {
			client.watched = make(map[string]int64)
		}

		for i := 1; i < len(args); i++ {
			key := args[i]
			client.watched[key] = client.Store.KeyVersion[key]
		}
		return "+OK\r\n"

	case "UNWATCH":
		client.watched = nil
		return "+OK\r\n"
	}

	// Inside MULTI -> queue instead of execute.
	if client.tx != nil {
		client.tx.commandqueue = append(client.tx.commandqueue, args)
		return "+QUEUED\r\n"
	}

	return ExecuteDirect(registry, args)
}

// encodeEXECArray wraps already-RESP-encoded replies in an outer array header.
func encodeEXECArray(input []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%d\r\n", len(input)))
	for _, val := range input {
		builder.WriteString(val) // already RESP formatted
	}
	return builder.String()
}
