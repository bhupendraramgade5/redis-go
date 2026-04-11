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

type Command interface {
	Execute(args []string) string
}

type ArityChecker interface {
	Arity() int
}

// PingCommand
type PingCommand struct{}

func (ping PingCommand) Execute(args []string) string {
	return "+PONG\r\n"
}

// EchoCommand
type EchoCommand struct{}

func (echo EchoCommand) Execute(args []string) string {
	return encodeBulkString(args[1])
}
func (echo EchoCommand) Arity() int {
	return 2
}

var internalmap = make(map[string]internalState)

type internalState struct {
	value     string
	expiresAt time.Time
}

type SetCommand struct {
}

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

	internalmap[key] = internalState{
		value:     value,
		expiresAt: expiresAt,
	}

	return "+OK\r\n"
}

type GetCommand struct{}

func (get GetCommand) Execute(args []string) string {
	key := args[1]

	state, ok := internalmap[key]
	if !ok {
		return "$-1\r\n"
	}

	if !state.expiresAt.IsZero() && time.Now().After(state.expiresAt) {
		delete(internalmap, key)
		return "$-1\r\n"
	}

	return encodeBulkString(state.value)
}

var Rpushmap = make(map[string]variables)

type variables struct {
	listmembers []string
}

type RpushCommand struct{}

func (rpush RpushCommand) Execute(args []string) string {
	key := args[1]
	var temp variables
	temp, ok := Rpushmap[key]

	if !ok {
		temp = variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.listmembers = append(temp.listmembers, args[i])
	}
	Rpushmap[key] = temp
	response := fmt.Sprintf(":%d\r\n", len(temp.listmembers))
	return response
}

type LRangeCommand struct{}

func (lrange LRangeCommand) Execute(args []string) string {
	key := args[1]
	lft, _ := strconv.Atoi(args[2])
	rgt, _ := strconv.Atoi(args[3])

	//Map Records : assignment and error handling
	temp, ok := Rpushmap[key]
	if !ok {
		return "*0\r\n"
	}

	size := len(temp.listmembers)

	// negative indices handling
	if lft < 0 {
		lft = size + lft%size
	}
	if rgt < 0 {
		rgt = size + rgt%size
	}

	if lft > rgt || lft >= size {
		return "*0\r\n"
	}

	var builder strings.Builder
	start:=lft
	end:=min(rgt, len(temp.listmembers)-1)
	builder.WriteString(fmt.Sprintf("*%d\r\n", end-start+1))
	// response:=fmt.Sprintf()
	for i := start; i <=end; i++ {
		val := temp.listmembers[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

var commands = map[string]Command{
	"PING":  PingCommand{},
	"ECHO":  EchoCommand{},
	"SET":   SetCommand{},
	"GET":   GetCommand{},
	"RPUSH": RpushCommand{},
	"LRANGE":LRangeCommand{},
}

func handleCommand(args []string) string {
	if len(args) == 0 {
		return "-ERR unknown command\r\n"
	}
	command := strings.ToUpper(args[0])
	handler := commands[command]

	if arityCmd, ok := handler.(ArityChecker); ok {
		if len(args) != arityCmd.Arity() {
			return "-ERR wrong number of arguments\r\n"
		}
	}
	return handler.Execute(args)
}
