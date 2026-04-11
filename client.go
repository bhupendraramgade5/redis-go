package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
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

		response := readRESP(serverReader)

		fmt.Print(response)
	}
}

func encodeRESP(input string) string {
	parts := strings.Fields(input)

	resp := fmt.Sprintf("*%d\r\n", len(parts))

	for _, part := range parts {
		resp += fmt.Sprintf("$%d\r\n%s\r\n", len(part), part)
	}

	return resp
}

func readRESP(reader *bufio.Reader) string {

	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	switch line[0] {

	case '+': // Simple string
		return line[1:]

	case '-': // Error
		return line

	case ':': // Integer
		return line

	case '$': // Bulk string
		return parseBulkString(reader, line)

	case '*': // Array
		return parseArray(reader, line)
	}

	return line
}

func parseBulkString(reader *bufio.Reader, line string) string {

	length, _ := strconv.Atoi(strings.TrimSpace(line[1:]))

	if length == -1 {
		return "(nil)\n"
	}

	data := make([]byte, length+2)
	reader.Read(data)

	return string(data[:length]) + "\n"
}

func parseArray(reader *bufio.Reader, line string) string {

	count, _ := strconv.Atoi(strings.TrimSpace(line[1:]))

	if count <= 0 {
		return "(empty list)\n"
	}

	var builder strings.Builder

	for i := 0; i < count; i++ {
		builder.WriteString(readRESP(reader))
	}

	return builder.String()
}
