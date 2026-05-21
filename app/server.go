package main

import (
	"fmt"
	"net"
)

// func consumeListener(l net.Listener, registry map[string]Command) {
func consumeListener(l net.Listener, registry map[string]Command, store *DataStore){
	// store := NewStore()
	for {
		connection, err := l.Accept()

		fmt.Println("Accepted new connection")

		if err != nil {
			fmt.Println("Error accepting connection:", err)
			return
		}

		go handleConnection(connection, registry, store)
	}
}

func handleConnection(connection net.Conn, registry map[string]Command, store *DataStore) {

	client := &Client{
		conn:  connection,
		store: store,
	}
	var buffer []byte // 🔥 persistent buffer
	fmt.Println("CLIENT PTR:", client)
	for {
		fmt.Println("CLIENT PTR:", client)

		temp := make([]byte, 1024)
		n, _ := connection.Read(temp)
		// if err != nil {
		//     return
		// }
		buffer = append(buffer, temp[:n]...)

		commands, remaining := parseRESP(buffer)
		buffer = remaining // keep leftover


		// for _, cmd := range commands {
		// 	response := handleCommand(client, registry, cmd)
		// 	connection.Write([]byte(response))
		// }

		for _, cmd := range commands {

			respChan := make(chan string)

			commandQueue <- Task{
				client:   client,
				command:  cmd,
				respChan: respChan,
			}

			response := <-respChan
			connection.Write([]byte(response))
		}
	}
}
