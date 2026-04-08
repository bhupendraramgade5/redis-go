package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	consumeListener(l)
}

func consumeListener(l net.Listener) {
	for {
		connection, err := l.Accept()

		fmt.Println("Accepted new connection")

		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		fmt.Println("Spawning handler for new connection")
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

func parseRESP(data []byte) []string {
	lines := strings.Split(string(data), "\r\n")

	var result []string

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "$") {
			length, _ := strconv.Atoi(lines[i][1:])
			if length > 0 && i+1 < len(lines) {
				result = append(result, lines[i+1])
				i++
			}
		}
	}

	return result
}

func handleCommand(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}

	switch strings.ToUpper(cmd[0]) {

	case "PING":
		return "+PONG\r\n"

	case "ECHO":
		if len(cmd) < 2 {
			return "-ERR wrong number of arguments\r\n"
		}
		return encodeBulkString(cmd[1])
	}

	return "-ERR unknown command\r\n"
}

func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}