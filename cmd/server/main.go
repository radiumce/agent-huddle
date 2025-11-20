package main

import (
	"flag"
	"log"

	"github.com/chene/agent-huddle/pkg/mcp"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on")
	flag.Parse()

	srv := mcp.NewServer()
	if err := srv.Start(*addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
