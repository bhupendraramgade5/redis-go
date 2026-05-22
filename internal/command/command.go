// Package command implements the Redis command handlers. It depends on the
// store package (for state) and the resp package (for wire encoding), placing
// it above both in the dependency graph. It depends on nothing in server —
// server depends on command, never the reverse. If you ever find yourself
// wanting command to import server, that is the compiler telling you a type is
// in the wrong layer.
package command

import (
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

// Command is the behavior every handler implements: turn parsed arguments into
// a RESP-encoded reply string.
type Command interface {
	Execute(args []string) string
}

// ArityChecker is implemented by commands that want the dispatcher to validate
// their argument count before Execute is called. It is a separate interface
// from Command (interface segregation) so handlers that do not care about arity
// are not forced to implement it.
type ArityChecker interface {
	Arity() int
}

// NewRegistry builds the command name -> handler map for a given store. Each
// handler closes over the same *store.DataStore, which is the one owned by the
// Server. (Previously the store was effectively global; now it is injected.)
func NewRegistry(s *store.DataStore) map[string]Command {
	return map[string]Command{
		"PING":   PingCommand{},
		"ECHO":   EchoCommand{},
		"SET":    SetCommand{Store: s},
		"GET":    GetCommand{Store: s},
		"RPUSH":  RpushCommand{Store: s},
		"LRANGE": LRangeCommand{Store: s},
		"LPUSH":  LPushCommand{Store: s},
		"LLEN":   LLenCommand{Store: s},
		"LPOP":   LPopCommand{Store: s},
		"BLPOP":  BLPopCommand{Store: s},
		"TYPE":   TYPECommand{Store: s},
		"XADD":   XADDCommand{Store: s},
		"XRANGE": XRANGECommand{Store: s},
		"XREAD":  XREADCommand{Store: s},
		"INCR":   INCRCommand{Store: s},
	}
}

// ExecuteDirect dispatches a single command against the registry, performing
// arity validation when the handler opts in. It is the path used both for
// normal commands and for commands replayed inside EXEC.
func ExecuteDirect(registry map[string]Command, args []string) string {
	if len(args) == 0 {
		return "-ERR unknown command\r\n"
	}
	name := strings.ToUpper(args[0])

	handler, ok := registry[name]
	if !ok {
		return "-ERR unknown command\r\n"
	}

	if arityCmd, ok := handler.(ArityChecker); ok {
		if len(args) != arityCmd.Arity() {
			return "-ERR wrong number of arguments\r\n"
		}
	}
	return handler.Execute(args)
}
