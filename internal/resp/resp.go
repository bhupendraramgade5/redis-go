// Package resp implements parsing and encoding for the Redis Serialization
// Protocol (RESP). It is the lowest layer in the project's dependency graph: it
// imports nothing from this module, only the standard library. Everything else
// (store, command, server) may depend on resp, but resp depends on nothing —
// which is exactly why it can be unit-tested in complete isolation.
package resp

import (
	"fmt"
)

// Parse scans a byte buffer that may contain zero or more complete RESP arrays
// (the wire form of client commands). It returns every fully-parsed command
// plus any trailing bytes that did not form a complete command yet, so the
// caller can prepend them to the next read. This is the mechanism that makes
// fragmented-TCP handling possible.
//
// NOTE: This is moved verbatim from the original app/resp.go to preserve
// behavior exactly during Phase 0. The suspicious slice arithmetic in the
// truncated-bulk-string branch is intentionally left as-is; it is a Phase 2/3
// target (fuzzing + invalid-input tables), not a Phase 0 change.
func Parse(data []byte) ([][]string, []byte) {
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
				return result, data[i-(j-i+1):]
			}

			arg := string(data[i : i+length])
			cmd = append(cmd, arg)

			i += length + 2 // skip \r\n
		}

		result = append(result, cmd)
	}

	return result, data[i:]
}

// EncodeBulkString returns the RESP bulk-string encoding of s.
func EncodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

// EncodeSimpleString returns the RESP simple-string encoding of s.
func EncodeSimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}
