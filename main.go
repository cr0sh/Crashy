package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <target address:port>", os.Args[0])
		os.Exit(0)
	}
	_, err := net.Dial("udp", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
}
