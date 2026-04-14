package main

import (
	// "fmt"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// May be there is a method which doesnt require the use of arity in future
// But to write implementation of that method we were still dependent on the
// Arity function
// thus by segregatin the interface we can now directky use the methods that we actually
// need, and saving efforts in writing methods that are unneccesary

var internalmap = make(map[string]internalState)

type internalState struct {
	value     string
	expiresAt time.Time
}

type variables struct {
	listleft []string
	listright []string
}

type DataStore struct {
	KV    map[string]internalState
	Lists map[string]variables
}

func NewStore() *DataStore {
	return &DataStore{
		KV:    make(map[string]internalState),
		Lists: make(map[string]variables),
	}
}

type Command interface {
	Execute(args []string) string
}

type ArityChecker interface {
	Arity() int
}

type PingCommand struct{}

type EchoCommand struct{}
type SetCommand struct {
	Store *DataStore
}
type GetCommand struct {
	Store *DataStore
}
type RpushCommand struct {
	Store *DataStore
}
type LRangeCommand struct {
	Store *DataStore
}
type LPushCommand struct {
	Store *DataStore
}

func NewRegistry(store *DataStore) map[string]Command {
	return map[string]Command{
		"PING":  PingCommand{},
		"ECHO":  EchoCommand{},
		"SET":   SetCommand{Store: store},
		"GET":   GetCommand{Store: store},
		"RPUSH": RpushCommand{Store: store},
		"LRANGE": LRangeCommand{Store: store},
		"LPUSH": LPushCommand{Store: store},
	}
}


func (ping PingCommand) Execute(args []string) string {
	return "+PONG\r\n"
}

func (echo EchoCommand) Execute(args []string) string {
	return encodeBulkString(args[1])
}
func (echo EchoCommand) Arity() int {
	return 2
}

// var internalmap = make(map[string]internalState)

// type internalState struct {
// 	value     string
// 	expiresAt time.Time
// }

func (set SetCommand) Execute(args []string) string {
	key := args[1]
	value := args[2]

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

	set.Store.KV[key] = internalState{
		value:     value,
		expiresAt: expiresAt,
	}

	return "+OK\r\n"
}

func (get GetCommand) Execute(args []string) string {
	key := args[1]

	state, ok := get.Store.KV[key]
	if !ok {
		return "$-1\r\n"
	}

	if !state.expiresAt.IsZero() && time.Now().After(state.expiresAt) {
		delete(internalmap, key)
		return "$-1\r\n"
	}

	return encodeBulkString(state.value)
}

// var Rpushmap = make(map[string]variables)

// type variables struct {
// 	listright []string
// }

func (rpush RpushCommand) Execute(args []string) string {
	key := args[1]
	var temp variables
	temp, ok := rpush.Store.Lists[key]

	if !ok {
		temp = variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.listright = append(temp.listright, args[i])
	}
	rpush.Store.Lists[key] = temp
	total := len(temp.listleft) + len(temp.listright)

	response := fmt.Sprintf(":%d\r\n", total)
	return response
}


func (lpush LPushCommand) Execute(args []string) string {
	key:=args[1]
	temp, ok :=lpush.Store.Lists[key]

	if !ok {
		temp = variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.listleft = append(temp.listleft, args[i])
	}
	
	lpush.Store.Lists[key] = temp
	total := len(temp.listleft) + len(temp.listright)

	response := fmt.Sprintf(":%d\r\n", total)
	return response
}


func (lrange LRangeCommand) Execute(args []string) string {
	key := args[1]
	lft, _ := strconv.Atoi(args[2])
	rgt, _ := strconv.Atoi(args[3])

	//Map Records : assignment and error handling
	temp, ok := lrange.Store.Lists[key]
	if !ok {
		return "*0\r\n"
	}

	combined := make([]string, 0)
	for i := len(temp.listleft)-1; i >= 0; i-- {
		combined = append(combined, temp.listleft[i])
	}
	combined = append(combined, temp.listright...)
	
	size := len(combined)
	// negative indices handling
	if lft < 0 {
		lft = size + lft
	}
	if rgt < 0 {
		rgt = size + rgt
	}
	if lft < 0 {
		lft = 0
	}
	if rgt < 0 {
		return "*0\r\n"
	}
	if rgt >= size {
		rgt = size - 1
	}

	if lft > rgt || lft >= size {
		return "*0\r\n"
	}

	var builder strings.Builder

	start := lft
	end := min(rgt, len(combined)-1)
	builder.WriteString(fmt.Sprintf("*%d\r\n", end-start+1))

	// response:=fmt.Sprintf()
	for i := start; i <= end; i++ {
		val := combined[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

func handleCommand(registry map[string]Command, args []string) string {
	if len(args) == 0 {
		return "-ERR unknown command\r\n"
	}
	command := strings.ToUpper(args[0])
	handler := registry[command]

	if arityCmd, ok := handler.(ArityChecker); ok {
		if len(args) != arityCmd.Arity() {
			return "-ERR wrong number of arguments\r\n"
		}
	}
	return handler.Execute(args)
}


