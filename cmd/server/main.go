// Command server is the production entrypoint. It constructs a single Server
// and runs it. All the state that used to be package-global (the command queue,
// the consumer goroutine, the store) now lives inside that Server value.
package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/redis-starter-go/internal/server"
)

func main() {
	s := server.New()
	if err := s.Start("0.0.0.0:6379"); err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
}
