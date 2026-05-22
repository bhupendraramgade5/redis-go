// Package server wires everything together: it owns a TCP listener, a single
// DataStore, the command registry, and the command queue plus its consumer
// goroutine. It is the top of the dependency graph — it imports command, store,
// and resp, and nothing imports it except cmd/server (the binary).
//
// Phase 0 is mostly about THIS file. In the original code, commandQueue and the
// goroutine that drained it were package-level globals in package main. That
// made it impossible to run two independent servers in one process (e.g. two
// integration tests in parallel) because they would share one queue and one
// consumer. Here, all of that state is owned by a Server value. Construct two
// Servers and they share nothing.
package server

import (
	"net"

	"github.com/codecrafters-io/redis-starter-go/internal/command"
	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

// task is one unit of work for the consumer goroutine: a parsed command for a
// particular client, plus a channel to deliver the reply on.
type task struct {
	client   *command.Client
	command  []string
	respChan chan string
}

// Server is a single, self-contained instance of the database server. All
// previously-global state lives here.
type Server struct {
	listener net.Listener
	store    *store.DataStore
	registry map[string]command.Command
	queue    chan task
	quit     chan struct{}
}

// New creates a Server with its own store, registry, and command queue. It does
// not start listening; call Start (or Listen + Serve) for that.
func New() *Server {
	st := store.New()
	return &Server{
		store:    st,
		registry: command.NewRegistry(st),
		queue:    make(chan task, 100),
		quit:     make(chan struct{}),
	}
}

// Store exposes the server's store. Tests and bootstrap helpers use this to
// seed or inspect state directly.
func (s *Server) Store() *store.DataStore { return s.store }

// Listen binds to addr (use "127.0.0.1:0" in tests to get an OS-assigned free
// port) and returns the actual address bound. It starts the consumer goroutine.
func (s *Server) Listen(addr string) (net.Addr, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.listener = l
	go s.consume()
	return l.Addr(), nil
}

// Serve accepts connections until the listener is closed. It blocks, so callers
// typically run it in a goroutine.
func (s *Server) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Accept fails when the listener is closed (Close was called) or on
			// a fatal error. Either way, stop accepting.
			return
		}
		go s.handleConnection(conn)
	}
}

// Start is the convenience path for production: bind to addr and serve until
// Close. main() uses this.
func (s *Server) Start(addr string) error {
	if _, err := s.Listen(addr); err != nil {
		return err
	}
	s.Serve()
	return nil
}

// Close stops accepting new connections and signals shutdown.
func (s *Server) Close() error {
	close(s.quit)
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// consume is the single consumer goroutine. It drains the per-Server queue and
// dispatches each task. This preserves the original single-consumer semantics
// exactly (including the deadlocks that follow from running blocking commands
// here) — Phase 0 changes structure, not behavior.
func (s *Server) consume() {
	for {
		select {
		case <-s.quit:
			return
		case t := <-s.queue:
			t.respChan <- s.dispatch(t)
		}
	}
}

// dispatch is the SEAM. Today it simply calls command.Handle on the consumer
// goroutine, identical to the old behavior. When Phase 3 addresses the
// single-consumer blocking deadlock, this is the one method to change (e.g. to
// run blocking commands off the consumer goroutine) without touching the
// command package or the connection loop.
func (s *Server) dispatch(t task) string {
	return command.Handle(t.client, s.registry, t.command)
}

// handleConnection reads from one client connection, parses RESP frames, and
// submits each parsed command to the queue, writing replies back in order.
//
// NOTE (left intact for later phases): the read error is still ignored, so a
// dropped connection spins this loop forever — the goroutine leak the roadmap
// pins in Phase 5. Preserved deliberately.
func (s *Server) handleConnection(conn net.Conn) {
	client := command.NewClient(s.store)
	var buffer []byte // persistent buffer across reads (fragmentation handling)

	for {
		temp := make([]byte, 1024)
		n, _ := conn.Read(temp)
		buffer = append(buffer, temp[:n]...)

		commands, remaining := resp.Parse(buffer)
		buffer = remaining

		for _, cmd := range commands {
			respChan := make(chan string)
			s.queue <- task{
				client:   client,
				command:  cmd,
				respChan: respChan,
			}
			response := <-respChan
			conn.Write([]byte(response))
		}
	}
}
