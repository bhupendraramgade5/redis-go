package main

import (
	"fmt"
	"net"
	"os"
)


type Task struct {
	client   *Client
	command  []string
	respChan chan string
}

var commandQueue = make(chan Task, 100)


func main() {
	store := NewStore()
	registry := NewRegistry(store)
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	
	go func() {
	for task := range commandQueue {
			response := handleCommand(task.client, registry, task.command)
			task.respChan <- response
		}
	}()

	// consumeListener(l, registry)
	consumeListener(l, registry, store)
}