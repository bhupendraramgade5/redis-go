package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type PingCommand struct{}

func (PingCommand) Execute(args []string) string { return "+PONG\r\n" }

type EchoCommand struct{}

func (EchoCommand) Execute(args []string) string { return resp.EncodeBulkString(args[1]) }
func (EchoCommand) Arity() int                   { return 2 }

type SetCommand struct{ Store *store.DataStore }

func (set SetCommand) Execute(args []string) string {
	key := args[1]
	value := args[2]

	set.Store.Lock()
	defer set.Store.Unlock()
	var expiresAt time.Time

	for i := 3; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			seconds, _ := strconv.Atoi(args[i+1])
			expiresAt = time.Now().Add(time.Duration(seconds) * time.Second)
			i++
		case "PX":
			ms, _ := strconv.Atoi(args[i+1])
			expiresAt = time.Now().Add(time.Duration(ms) * time.Millisecond)
			i++
		}
	}

	set.Store.KV[key] = store.InternalState{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	set.Store.KeyVersion[key]++

	return "+OK\r\n"
}

type GetCommand struct{ Store *store.DataStore }

func (get GetCommand) Execute(args []string) string {
	key := args[1]

	get.Store.Lock()
	defer get.Store.Unlock()
	state, ok := get.Store.KV[key]
	if !ok {
		return "$-1\r\n"
	}

	if !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt) {
		delete(get.Store.KV, key)
		get.Store.KeyVersion[key]++
		return "$-1\r\n"
	}

	return resp.EncodeBulkString(state.Value)
}

type INCRCommand struct{ Store *store.DataStore }

func (incr INCRCommand) Execute(args []string) string {
	key := args[1]
	incr.Store.Lock()
	defer incr.Store.Unlock()

	state, ok := incr.Store.KV[key]

	// Key does not exist: add it and return.
	if !ok {
		incr.Store.KV[key] = store.InternalState{Value: "1"}
		return ":1\r\n"
	}

	if !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt) {
		delete(incr.Store.KV, key)
		incr.Store.KV[key] = store.InternalState{Value: "1"}
		incr.Store.KeyVersion[key]++
		return ":1\r\n"
	}

	num, err := strconv.Atoi(state.Value)
	if err != nil {
		return "-ERR value is not an integer or out of range\r\n"
	}

	num++

	state.Value = strconv.Itoa(num)
	incr.Store.KV[key] = state
	incr.Store.KeyVersion[key]++

	return fmt.Sprintf(":%d\r\n", num)
}
