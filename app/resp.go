package main

import (
	"fmt"
	// "strconv"
	// "strings"
)


func parseRESP(data []byte) ([][]string, []byte) {
    var result [][]string
    i := 0

    for i < len(data) {
        // Need at least "*<num>\r\n"
        if data[i] != '*' {
            break
        }

        // Find end of line
        j := i
        for j < len(data) && data[j] != '\n' {
            j++
        }
        if j >= len(data) {
            break // incomplete
        }

        var numArgs int
        fmt.Sscanf(string(data[i+1:j-1]), "%d", &numArgs)

        i = j + 1
        var cmd []string

        for k := 0; k < numArgs; k++ {
            if i >= len(data) || data[i] != '$' {
                return result, data[i:]
            }

            // Read length
            j = i
            for j < len(data) && data[j] != '\n' {
                j++
            }
            if j >= len(data) {
                return result, data[i:]
            }

            var length int
            fmt.Sscanf(string(data[i+1:j-1]), "%d", &length)

            i = j + 1

            // Check if full data available
            if i+length+2 > len(data) {
                return result, data[i- (j - i + 1):]
            }

            arg := string(data[i : i+length])
            cmd = append(cmd, arg)

            i += length + 2 // skip \r\n
        }

        result = append(result, cmd)
    }

    return result, data[i:]
}


func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func encodeSimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}