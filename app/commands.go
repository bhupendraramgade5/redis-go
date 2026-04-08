package main

import (
	"fmt"
	"strings"
)

// May be there is a method which doesnt require the use of arity in future
// But to write implementation of that method we were still dependent on the
// Arity function
// thus by segregatin the interface we can now directky use the methods that we actually
// need, and saving efforts in writing methods that are unneccesary

type Command interface{
	Execute(args []string) string
}

type ArityChecker interface{
	Arity() int
}

// PingCommand
type PingCommand struct{}
func (ping PingCommand)  Execute(args []string) string {
	return "+PONG\r\n"
}



//EchoCommand
type EchoCommand struct{}

func (echo EchoCommand) Execute(args[] string) string{
	return encodeBulkString(args[1])	
}

func (echo EchoCommand) Arity() int {
	return 2
}

var internalmap map[string]string

// type Server struct {
// 	store map[string]string
// }
type SetCommand struct{
}
func (set SetCommand) Execute(args[] string) string{
	internalmap[args[1]]=args[2]
	return "+OK\r\n"
}
type GetCommand struct{}

func (get GetCommand) Execute(args [] string)string{
	value, ok:=internalmap[args[1]]
	if ok {
		var response string 
		response=fmt.Sprintf("$%d\r\n%s\r\n", len(value),value) // RESP response type bulk string 
		return response 
	}else{
		return "-1\r\n" // Null Bulk String :: special type
	}
}

var commands = map[string]Command{
	"PING": PingCommand{},
	"ECHO": EchoCommand{},
	"SET": SetCommand{},
	"GET" : GetCommand{},
}

func handleCommand(args []string )string {
	if len(args)==0 {
		return "-ERR unknown command\r\n"
	}
	command:=strings.ToUpper(args[0])
	// handler, ok:=commands[command]

	handler := commands[command]

	if arityCmd, ok := handler.(ArityChecker); ok {
		if len(args) != arityCmd.Arity() {
			return "-ERR wrong number of arguments\r\n"
		}
	}

	// return handler.Execute(cmd)

	return handler.Execute(args)
}