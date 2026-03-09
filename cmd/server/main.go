package main

import (
	"flag"
	"log"

	"github.com/chene/agent-huddle/pkg/api"
	"github.com/chene/agent-huddle/pkg/huddle"
	"github.com/chene/agent-huddle/pkg/mcp"
)

func main() {
	addr := flag.String("addr", ":8880", "Address to listen on (MCP Server)")
	apiAddr := flag.String("api-addr", ":8881", "Address of HTTP API logic")
	flag.Parse()

	service := huddle.NewService(nil)

	// Start API server in goroutine
	apiSrv := api.NewServer(service)
	go func() {
		log.Printf("HTTP API server listening on %s", *apiAddr)
		if err := apiSrv.Start(*apiAddr); err != nil {
			log.Fatalf("API Server failed: %v", err)
		}
	}()

	srv := mcp.NewServer(service)
	if err := srv.Start(*addr); err != nil {
		log.Fatalf("MCP Server failed: %v", err)
	}
}
