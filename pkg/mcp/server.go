package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/chene/agent-huddle/pkg/room"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/server"
)

type HuddleServer struct {
	manager *room.Manager
	mcp     *server.Server
}

func NewHuddleServer() *HuddleServer {
	s := &HuddleServer{
		manager: room.NewManager(),
	}
	
	s.mcp = server.NewServer(server.Info{
		Name:    "agent-huddle",
		Version: "0.1.0",
	})

	s.registerTools()
	return s
}

func (s *HuddleServer) ServeHTTP(w interface{}, r interface{}) {
	// Placeholder for HTTP serving logic if using standard net/http
}

func (s *HuddleServer) GetMCPServer() *server.Server {
	return s.mcp
}

func (s *HuddleServer) registerTools() {
	s.mcp.AddTool(mcp.Tool{
		Name:        "create_room",
		Description: "Create a new meeting room. Returns room ID.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]string{"type": "string", "description": "Name of the meeting"},
			},
			Required: []string{"name"},
		},
	}, s.handleCreateRoom)

	s.mcp.AddTool(mcp.Tool{
		Name:        "list_rooms",
		Description: "List all active meeting rooms.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
		},
	}, s.handleListRooms)

	s.mcp.AddTool(mcp.Tool{
		Name:        "join_room",
		Description: "Join a meeting room.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"room_id":   map[string]string{"type": "string"},
				"member_id": map[string]string{"type": "string"},
				"role":      map[string]string{"type": "string", "enum": "host,participant"},
			},
			Required: []string{"room_id", "member_id", "role"},
		},
	}, s.handleJoinRoom)

	s.mcp.AddTool(mcp.Tool{
		Name:        "send_message",
		Description: "Send a message to the room. Participants may experience a delay to check for Host priority.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"room_id":   map[string]string{"type": "string"},
				"member_id": map[string]string{"type": "string"},
				"content":   map[string]string{"type": "string"},
			},
			Required: []string{"room_id", "member_id", "content"},
		},
	}, s.handleSendMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "poll_messages",
		Description: "Wait for new messages in the room (long polling).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"room_id":     map[string]string{"type": "string"},
				"member_id":   map[string]string{"type": "string"},
				"last_msg_id": map[string]interface{}{"type": "integer"}, // Use interface{} for flexibility
				"timeout_sec": map[string]interface{}{"type": "integer"},
			},
			Required: []string{"room_id", "member_id", "last_msg_id"},
		},
	}, s.handlePollMessages)

	s.mcp.AddTool(mcp.Tool{
		Name:        "wait",
		Description: "Wait for a specified duration.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"duration_sec": map[string]interface{}{"type": "integer"},
			},
			Required: []string{"duration_sec"},
		},
	}, s.handleWait)

	s.mcp.AddTool(mcp.Tool{
		Name:        "close_room",
		Description: "Close the meeting room (Host only).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"room_id":   map[string]string{"type": "string"},
				"member_id": map[string]string{"type": "string"},
			},
			Required: []string{"room_id", "member_id"},
		},
	}, s.handleCloseRoom)
}

func (s *HuddleServer) handleCreateRoom(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid name")
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	r, err := s.manager.CreateRoom(id, name)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Room created. ID: %s", r.ID)}},
	}, nil
}

func (s *HuddleServer) handleListRooms(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	rooms := s.manager.ListRooms()
	var text string
	for _, r := range rooms {
		text += fmt.Sprintf("ID: %s, Name: %s\n", r.ID, r.Name)
	}
	if text == "" {
		text = "No active rooms."
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: text}},
	}, nil
}

func (s *HuddleServer) handleJoinRoom(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	roomID, _ := args["room_id"].(string)
	memberID, _ := args["member_id"].(string)
	roleStr, _ := args["role"].(string)

	r, err := s.manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	role := room.RoleParticipant
	if roleStr == "host" {
		role = room.RoleHost
	}

	if err := r.Join(memberID, role); err != nil {
		return nil, err
	}

	msgs, _ := r.GetMessages(ctx, 0, 0)
	var history string
	for _, m := range msgs {
		history += fmt.Sprintf("[%s] %s: %s\n", m.Timestamp.Format(time.RFC3339), m.SenderID, m.Content)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Joined room %s.\nHistory:\n%s", roomID, history)}},
	}, nil
}

func (s *HuddleServer) handleSendMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	roomID, _ := args["room_id"].(string)
	memberID, _ := args["member_id"].(string)
	content, _ := args["content"].(string)

	r, err := s.manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	role := r.GetMemberRole(memberID)
	if role == "" {
		return nil, fmt.Errorf("member not found in room")
	}

	if role == room.RoleParticipant {
		// Host Priority Logic
		// Wait 500ms
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		// Check if host sent a message recently (e.g. in the last 1 second)
		latest := r.GetLatestMessage()
		if latest != nil {
			// We need to check if the sender was a host.
			// This requires looking up the sender's role.
			senderRole := r.GetMemberRole(latest.SenderID)
			if senderRole == room.RoleHost && time.Since(latest.Timestamp) < 1*time.Second {
				return &mcp.CallToolResult{
					Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Host sent a message: %s. Please review before sending.", latest.Content)}},
					IsError: true, // Indicate retry/failure
				}, nil
			}
		}
	}

	msgID, err := r.SendMessage(memberID, content, "")
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Message sent. ID: %d", msgID)}},
	}, nil
}

func (s *HuddleServer) handlePollMessages(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	roomID, _ := args["room_id"].(string)
	memberID, _ := args["member_id"].(string) // Not strictly used for polling logic but good for tracking
	
	// Handle different number types from JSON
	var lastMsgID int64
	switch v := args["last_msg_id"].(type) {
	case float64:
		lastMsgID = int64(v)
	case int:
		lastMsgID = int64(v)
	case string:
		fmt.Sscanf(v, "%d", &lastMsgID)
	}

	var timeoutSec int
	switch v := args["timeout_sec"].(type) {
	case float64:
		timeoutSec = int(v)
	case int:
		timeoutSec = v
	default:
		timeoutSec = 30 // Default
	}

	r, err := s.manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	// Update member's last seen message
	r.UpdateMemberLastMsgID(memberID, lastMsgID)

	msgs, err := r.GetMessages(ctx, lastMsgID, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: "No new messages"}},
		}, nil
	}

	var output string
	var newLastID int64
	for _, m := range msgs {
		output += fmt.Sprintf("ID: %d, Sender: %s, Content: %s\n", m.ID, m.SenderID, m.Content)
		if m.ID > newLastID {
			newLastID = m.ID
		}
	}
	
	// Update again
	r.UpdateMemberLastMsgID(memberID, newLastID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: output}},
	}, nil
}

func (s *HuddleServer) handleWait(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	var durationSec int
	switch v := args["duration_sec"].(type) {
	case float64:
		durationSec = int(v)
	case int:
		durationSec = v
	default:
		durationSec = 1
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(durationSec) * time.Second):
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Waited %d seconds", durationSec)}},
	}, nil
}

func (s *HuddleServer) handleCloseRoom(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	roomID, _ := args["room_id"].(string)
	memberID, _ := args["member_id"].(string)

	r, err := s.manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	role := r.GetMemberRole(memberID)
	if role != room.RoleHost {
		return nil, fmt.Errorf("only host can close the room")
	}

	s.manager.CloseRoom(roomID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: "Room closed"}},
	}, nil
}
