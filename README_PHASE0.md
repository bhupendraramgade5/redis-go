# Phase 0 — Restructured for Testability

This is your project after Phase 0: split from a single `package main` into layered
packages, with the command queue and consumer goroutine moved off package globals
and into a `Server` struct. **Behavior is unchanged** — the known bugs (EXEC
self-deadlock, BLPOP/XREAD blocking the consumer, the disconnect goroutine leak)
are intact on purpose, so the Phase 3 / Phase 5 tests can pin them.

## Folder structure and what moved where

```
redis-server/
├── go.mod                              # module root (unchanged)
├── cmd/
│   └── server/
│       └── main.go                     # WAS: app/main.go
│                                       #   now ~10 lines: construct a Server, Start it.
│                                       #   the global commandQueue + consumer goroutine
│                                       #   that lived here are GONE (moved into Server).
├── internal/
│   ├── resp/
│   │   └── resp.go                     # WAS: app/resp.go
│   │                                   #   Parse(), EncodeBulkString(), EncodeSimpleString().
│   │                                   #   Lowest layer — imports nothing internal.
│   ├── store/
│   │   └── store.go                    # WAS: the state types inside app/commands.go
│   │                                   #   DataStore, InternalState, Variables, Stream,
│   │                                   #   StreamEntry. Fields exported where the command
│   │                                   #   layer needs them; mutex hidden behind Lock/Unlock.
│   ├── command/
│   │   ├── command.go                  # Command/ArityChecker interfaces, NewRegistry,
│   │   │                               #   ExecuteDirect (WAS: executeDirect).
│   │   ├── string.go                   # PING ECHO SET GET INCR
│   │   ├── list.go                     # RPUSH LPUSH LPOP BLPOP LLEN LRANGE + encodeArray
│   │   ├── stream.go                   # XADD XRANGE XREAD TYPE + stream helpers
│   │   ├── tx.go                       # Client, TxContext, Handle (WAS: handleCommand)
│   │   │                               #   — MULTI/EXEC/WATCH/UNWATCH/DISCARD
│   │   └── command_test.go             # white-box round-trip unit test
│   └── server/
│       └── server.go                   # WAS: app/server.go + app/main.go globals
│                                       #   Server struct OWNS: listener, store, registry,
│                                       #   queue, consumer goroutine. handleConnection
│                                       #   and the consumer loop live here.
└── testutil/
    ├── server.go                       # StartServer (random port) + Dial helpers
    └── roundtrip_test.go               # black-box TCP test + two-server isolation test
```

The original `client.go` (the interactive REPL client) is a separate `package main`
program; it can move to `cmd/cli/main.go` later. It is not part of the server build and
is omitted here to keep the module single-binary for now. `app/utils.go_f` was empty.

## The dependency direction (the actual lesson)

Arrows point "depends on". Notice it's a DAG with no cycles — that is what makes each
layer testable in isolation:

```
        cmd/server  (main)
             │
             ▼
        internal/server ──────────────┐
             │                         │
             ▼                         ▼
        internal/command ──────► internal/resp
             │                         ▲
             ▼                         │
        internal/store ───────────────┘
             ▲
             │
         testutil (test-only) ──► internal/server
```

- `resp` depends on nothing internal → unit-testable with zero setup.
- `store` depends on nothing internal (only stdlib) → construct one and poke it.
- `command` depends on `store` + `resp` → inject a fresh `store.New()` per test.
- `server` depends on all three → the only layer that needs a real socket.
- Nothing depends on `server` except the binary and `testutil` → swapping the
  dispatch model later touches one package.

If you ever try to make a lower layer import a higher one (e.g. `store` importing
`command`), Go gives you an **import cycle** error. That error is the compiler
teaching you the type is in the wrong place — almost always the fix is "this type
belongs in the lower layer." Don't fight it; follow it.

## Why de-globalizing the queue matters (concretely)

Old design: `var commandQueue = make(chan Task, 100)` at package scope, drained by a
single goroutine started in `main()`. Two servers in one process = one shared queue =
no isolation. You literally cannot start a second independent server for a test.

New design: `Server` owns `queue chan task` and starts its own consumer in `Listen`.
`testutil.StartServer` constructs a fresh `Server` each call. `TestTwoServers_AreIsolated`
proves it: a key set on server A is invisible on server B. That test could not even be
*written* before.

## The seam for the future fix (no behavior change yet)

`server.go` routes every command through one method:

```go
func (s *Server) dispatch(t task) string {
    return command.Handle(t.client, s.registry, t.command)
}
```

Today this runs on the single consumer goroutine, exactly like before — including the
deadlocks. When Phase 3 addresses the single-consumer blocking problem, `dispatch` is
the one place to change (e.g. detect blocking commands and run them off the consumer).
The command package and the connection loop stay untouched. That's the payoff of the
clean layering.

## Build & test

```bash
go build ./...                       # compiles everything
go vet ./...                         # static checks
go test ./...                        # white-box + TCP round-trip + isolation tests
go test -race ./...                  # same, under the race detector
gofmt -l .                           # should print nothing (all formatted)
```

Expected: all tests green. They only prove the structure compiles and wires together —
breadth coverage is Phase 2, not now.

## What was deliberately NOT done (don't optimize early)

- No interfaces introduced speculatively (no `Store` interface, no `Clock` yet — Clock
  arrives in Phase 1 when expiry tests need determinism).
- No graceful shutdown beyond a minimal `Close` (Phase 5 concern).
- Package boundaries are *workable*, not *perfect*; they'll clarify over Phases 2–4.
- The bugs are untouched. Phase 0 changes structure, not behavior.

## One honesty note

This code was written and statically audited (imports, identifier renames, registry/type
consistency, dependency acyclicity, gofmt alignment) but **not compiled in the authoring
environment** — the Go toolchain wasn't available there. It is designed to compile and
pass on Go 1.26. If `go build ./...` surfaces anything, it'll almost certainly be a trivial
import or name fix; tell me the exact error text and I'll correct it.
