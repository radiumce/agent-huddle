package mcp

import (
	"log"
	"time"

	"github.com/chene/agent-huddle/pkg/huddle"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	service *huddle.Service
	mcpSrv  *server.MCPServer
}

func NewServer(service *huddle.Service) *Server {
	if service == nil {
		service = huddle.NewService(nil)
	}
	return &Server{
		service: service,
	}
}

func (s *Server) Start(addr string) error {
	s.mcpSrv = server.NewMCPServer(
		"agent-huddle",
		"0.1.0",
		server.WithToolCapabilities(true),
	)
	s.registerTools()

	// Create the streamable HTTP server with stateful session support
	httpServer := server.NewStreamableHTTPServer(s.mcpSrv,
		server.WithHeartbeatInterval(30*time.Second),
		server.WithStateLess(false), // Stateful connections
	)

	log.Printf("MCP server listening on %s (HTTP Streamable)", addr)
	return httpServer.Start(addr)
}
