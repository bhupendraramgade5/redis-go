package main

import "strings"

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