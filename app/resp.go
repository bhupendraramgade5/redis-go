package main

import (
	"fmt"
	"strconv"
	"strings"
)

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

func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}