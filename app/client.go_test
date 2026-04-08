package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {

	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		fmt.Println("Failed to connect to server")
		return
	}

	fmt.Println("Connected to Redis server")

	reader := bufio.NewReader(os.Stdin)
	serverReader := bufio.NewReader(conn)

	for {
		fmt.Print("> ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		resp := encodeRESP(input)

		conn.Write([]byte(resp))

		response := readResponse(serverReader)
		// fmt.Print(response)

		fmt.Print(parseResponse(response))
	}
}

func encodeRESP(input string) string {
	parts := strings.Split(input, " ")

	resp := fmt.Sprintf("*%d\r\n", len(parts))

	for _, part := range parts {
		resp += fmt.Sprintf("$%d\r\n%s\r\n", len(part), part)
	}

	return resp
}

func parseResponse(resp string) string {

	if strings.HasPrefix(resp, "+") {
		return resp[1:]
	}

	if strings.HasPrefix(resp, "$") {
		return resp
	}

	return resp
}

func readResponse(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')

	switch line[0] {

	case '+':
		return line[1:]

	case '-':
		return line

	case '$':
		var length int
		fmt.Sscanf(line, "$%d", &length)

		if length == -1 {
			return "(nil)\n"
		}

		data := make([]byte, length+2)
		reader.Read(data)

		return string(data[:length]) + "\n"
	}

	return line
}