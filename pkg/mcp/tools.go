package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/chene/agent-huddle/pkg/huddle"
)

// Input/Output structs

type CreateRoomInput struct {
	Name string `json:"name" jsonschema:"Name of the meeting room"`
	Host string `json:"host" jsonschema:"Name of the host agent"`
}

type CreateRoomOutput struct {
	Result string `json:"result"`
}

type JoinRoomInput struct {
	RoomNumber string `json:"room_number" jsonschema:"Room number to join"`
	MemberName string `json:"member_name" jsonschema:"Name of the joining agent"`
}

type JoinRoomOutput struct {
	Result string `json:"result"`
}

type PostMessageInput struct {
	RoomID     string `json:"room_id" jsonschema:"ID of the room"`
	Sender     string `json:"sender" jsonschema:"Name of the sender"`
	Content    string `json:"content" jsonschema:"Message content"`
	Recipient  string `json:"recipient,omitempty" jsonschema:"Recipient name (optional)"`
	LastSeenID int64  `json:"last_seen_id" jsonschema:"ID of the last message seen by the sender"`
}

type PostMessageOutput struct {
	Result string `json:"result"`
}

type WaitForMessageInput struct {
	RoomID     string `json:"room_id" jsonschema:"ID of the room"`
	MemberName string `json:"member_name" jsonschema:"Name of the waiting member"`
	LastMsgID  int64  `json:"last_msg_id" jsonschema:"ID of the last message received"`
	TimeoutSec int    `json:"timeout_sec" jsonschema:"Timeout in seconds"`
}

type WaitForMessageOutput struct {
	Messages []huddle.Message `json:"messages"`
}

type ListRoomsInput struct{}

type ListRoomsOutput struct {
	Rooms []*huddle.Room `json:"rooms"`
}

type CloseRoomInput struct {
	RoomID string `json:"room_id" jsonschema:"ID of the room"`
}

type CloseRoomOutput struct {
	Result string `json:"result"`
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "create_room", Description: "Create a new meeting room"}, s.handleCreateRoom)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "join_room", Description: "Join an existing meeting room"}, s.handleJoinRoom)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "post_message", Description: "Post a message to the room"}, s.handlePostMessage)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "wait_for_message", Description: "Wait for new messages in the room"}, s.handleWaitForMessage)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "list_rooms", Description: "List active meeting rooms"}, s.handleListRooms)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "close_room", Description: "Close a meeting room"}, s.handleCloseRoom)
}

func (s *Server) handleCreateRoom(ctx context.Context, req *mcp.CallToolRequest, input CreateRoomInput) (*mcp.CallToolResult, CreateRoomOutput, error) {
	room, err := s.manager.CreateRoom(input.Name, input.Host)
	if err != nil {
		return nil, CreateRoomOutput{}, err
	}
	return nil, CreateRoomOutput{Result: fmt.Sprintf("Room created. ID: %s, Number: %s", room.ID, room.Number)}, nil
}

func (s *Server) handleJoinRoom(ctx context.Context, req *mcp.CallToolRequest, input JoinRoomInput) (*mcp.CallToolResult, JoinRoomOutput, error) {
	room, err := s.manager.GetRoomByNumber(input.RoomNumber)
	if err != nil {
		return nil, JoinRoomOutput{}, err
	}

	member := &huddle.Member{
		Name:         input.MemberName,
		IsHost:       false,
		Active:       true,
		LastActiveAt: time.Now(),
	}
	room.Join(member)

	return nil, JoinRoomOutput{Result: fmt.Sprintf("Joined room %s (ID: %s). Please call wait_for_message with last_msg_id=0 to get history.", room.Name, room.ID)}, nil
}

func (s *Server) handlePostMessage(ctx context.Context, req *mcp.CallToolRequest, input PostMessageInput) (*mcp.CallToolResult, PostMessageOutput, error) {
	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, PostMessageOutput{}, err
	}

	err = room.PostMessage(input.Sender, input.Content, input.Recipient, input.LastSeenID)
	if err != nil {
		if err == huddle.ErrContextChanged {
			return nil, PostMessageOutput{}, fmt.Errorf("Context changed. Please fetch new messages.")
		}
		return nil, PostMessageOutput{}, err
	}

	return nil, PostMessageOutput{Result: "Message posted"}, nil
}

func (s *Server) handleWaitForMessage(ctx context.Context, req *mcp.CallToolRequest, input WaitForMessageInput) (*mcp.CallToolResult, WaitForMessageOutput, error) {
	timeout := time.Duration(input.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, WaitForMessageOutput{}, err
	}

	msgs, err := room.WaitForMessage("", input.LastMsgID, timeout)
	if err != nil {
		return nil, WaitForMessageOutput{}, err
	}

	return nil, WaitForMessageOutput{Messages: msgs}, nil
}

func (s *Server) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, input ListRoomsInput) (*mcp.CallToolResult, ListRoomsOutput, error) {
	rooms := s.manager.ListRooms()
	return nil, ListRoomsOutput{Rooms: rooms}, nil
}

func (s *Server) handleCloseRoom(ctx context.Context, req *mcp.CallToolRequest, input CloseRoomInput) (*mcp.CallToolResult, CloseRoomOutput, error) {
	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, CloseRoomOutput{}, err
	}
	room.Close()
	return nil, CloseRoomOutput{Result: "Room closed"}, nil
}
