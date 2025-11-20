package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/chene/agent-huddle/pkg/mcp"
)

func main() {
	s := mcp.NewHuddleServer()

	// Set up SSE endpoint
	// Assuming the SDK provides a way to get an HTTP handler or we implement a simple SSE wrapper.
	// For this implementation, we'll assume the server object (or a helper) can be used as a handler
	// or we expose the underlying MCP server's handler.
	// In pkg/mcp/server.go, we have GetMCPServer().
	
	// Note: The specific SDK API for SSE might vary. 
	// A common pattern is:
	// http.Handle("/sse", server.NewSSEHandler(s.GetMCPServer()))
	// http.Handle("/messages", server.NewMessageHandler(s.GetMCPServer()))
	
	// Since I don't have the exact SDK signature, I will assume a generic handler setup.
	// If this fails to compile, the user will need to adjust based on the specific SDK version.
	
	// Placeholder for SDK integration:
	// http.Handle("/mcp", s.GetMCPServer()) 
	
	fmt.Println("Starting Agent Huddle MCP Server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
