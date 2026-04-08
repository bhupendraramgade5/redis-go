package main

import (
	"fmt"
	"net"
)

func consumeListener(l net.Listener) {
	for {
		connection, err := l.Accept()

		fmt.Println("Accepted new connection")

		if err != nil {
			fmt.Println("Error accepting connection:", err)
			return
		}

		go handleConnection(connection)
	}
}

func handleConnection(connection net.Conn) {
	for {
		buf := make([]byte, 1024)

		n, err := connection.Read(buf)
		if err != nil {
			fmt.Println("Connection closed")
			return
		}

		command := parseRESP(buf[:n])
		response := handleCommand(command)

		connection.Write([]byte(response))
	}
}