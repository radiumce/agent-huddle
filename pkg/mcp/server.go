package mcp

import (
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/chene/agent-huddle/pkg/huddle"
)

type Server struct {
	manager *huddle.Manager
	mcpSrv  *mcp.Server
}

func NewServer() *Server {
	return &Server{
		manager: huddle.NewManager(),
	}
}

func (s *Server) Start(addr string) error {
	impl := &mcp.Implementation{
		Name:    "agent-huddle",
		Version: "0.1.0",
	}
	
	s.mcpSrv = mcp.NewServer(impl, nil)
	s.registerTools()
	
	// Create the streamable HTTP handler.
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpSrv
	}, nil)

	log.Printf("MCP server listening on %s (HTTP)", addr)
	return http.ListenAndServe(addr, handler)
}
