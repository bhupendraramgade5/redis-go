package main

import (
	"fmt"
	"net"
)

func consumeListener(l net.Listener, registry map[string]Command) {
	store := NewStore()
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

func handleConnection(connection net.Conn, registry map[string]Command,  store *DataStore) {
	client := &Client{
					conn: connection,
					store: store,
					}

	for {
		buf := make([]byte, 1024)

		n, err := connection.Read(buf)
		if err != nil {
			fmt.Println("Connection closed")
			return
		}

		command := parseRESP(buf[:n])
		response := handleCommand(client, registry, command)

		connection.Write([]byte(response))
	}
}