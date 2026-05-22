// Package store holds the server's in-memory state and the types that represent
// it: the key/value space, lists, streams, blocking waiters, and the key
// version counters used by WATCH. It sits one layer above resp and below
// command and server.
//
// Design note for Phase 0: the command layer lives in a *different* package and
// therefore cannot reach into unexported fields here. Rather than export every
// field (which would destroy encapsulation), we export the struct types that
// must cross the boundary and expose the few fields/locks the command layer
// genuinely needs. The fields the command code mutates directly (KV, Lists,
// DataStream, ListWaiters, StreamWaiters, KeyVersion) are exported; the
// synchronization primitive is exposed through Lock/Unlock methods. This is the
// minimal seam — we are deliberately NOT introducing repository interfaces or
// abstractions we do not yet need (rule of three; extract later).
package store

import (
	"sync"
	"time"
)

// InternalState is the stored representation of a string key.
type InternalState struct {
	Value     string
	ExpiresAt time.Time
}

// Variables is a list value, split into a left and right segment to make
// LPUSH/RPUSH O(1) at both ends (mirrors the original implementation).
type Variables struct {
	ListLeft  []string
	ListRight []string
}

// StreamEntry is a single entry in a stream.
type StreamEntry struct {
	ID     string
	Fields []string
	Time   int64
	Seq    int64
}

// Stream is an append-only log of entries plus a cursor to the top ID.
type Stream struct {
	Entries   []StreamEntry
	TopIDTime int64
	TopIDSeq  int64
}

// DataStore is the complete shared state of one server instance.
//
// IMPORTANT (Phase 0): there is exactly ONE DataStore per Server now, owned by
// the Server struct rather than created as a package global. That ownership is
// what lets two test servers run in the same process without sharing state.
type DataStore struct {
	KV            map[string]InternalState
	Lists         map[string]Variables
	ListWaiters   map[string][]chan []string
	StreamWaiters map[string][]chan struct{}
	DataStream    map[string]*Stream
	KeyVersion    map[string]int64

	mu sync.Mutex
}

// New constructs an empty DataStore with all maps initialized.
func New() *DataStore {
	return &DataStore{
		KV:            make(map[string]InternalState),
		Lists:         make(map[string]Variables),
		ListWaiters:   make(map[string][]chan []string),
		StreamWaiters: make(map[string][]chan struct{}),
		DataStream:    make(map[string]*Stream),
		KeyVersion:    make(map[string]int64),
	}
}

// Lock acquires the store's mutex. It is exported (rather than embedding
// sync.Mutex) so the command package can guard critical sections while the
// mutex field itself stays unexported and cannot be copied or replaced.
func (d *DataStore) Lock() { d.mu.Lock() }

// Unlock releases the store's mutex.
func (d *DataStore) Unlock() { d.mu.Unlock() }
