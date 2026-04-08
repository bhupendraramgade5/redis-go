package main

import "strings"

// May be there is a method which doesnt require the use of arity in future 
// But to write implementation of that method we were still dependent on the 
// Arity function
// thus by segregatin the interface we can now directky use the methods that we actually 
// need, and saving efforts in writing methods that are unneccesary

type Command interface{
	Execute(args []string) string
}

type Arity interface{
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


type SetCommand struct{}
func (set SetCommand) Execute(args[] string) string{

}


var commands = map[string]Command{
	"PING": PingCommand{},
	"ECHO": EchoCommand{},
	"SET": SetCommand{},
}

func handleCommand(args []string )string {
	if len(args)==0 {
		return "-ERR unknown command\r\n"
	}
	command:=strings.ToUpper(args[0])
	handler, ok:=commands[command]
	if !ok{
		return "-ERR unknown command\r\n"
	}
	if len(args)!=handler.Arity(){
		return "-ERR wrong number of arguments\r\n"
	}
	return handler.Execute(args)
}