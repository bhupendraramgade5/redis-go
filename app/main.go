package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"strconv"
)

func main() {

	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	consumeListener(l)
}