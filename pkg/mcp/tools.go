package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/chene/agent-huddle/pkg/huddle"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const DefaultTimeoutSec = 600

// Input/Output structs

type CreateRoomAndWaitInput struct {
	RoomID      string `json:"room_id" jsonschema:"Unique ID for the room (required)"`
	Name        string `json:"name" jsonschema:"Name of the meeting room"`
	Host        string `json:"host" jsonschema:"Name of the host agent"`
	InitMessage string `json:"init_message,omitempty" jsonschema:"Optional initial message to post"`
	TimeoutSec  int    `json:"timeout_sec" jsonschema:"Optional, this service is based on the long polling mechanism, and can accept a longer timeout(default 600s), no need to modify it unless necessary"`
}

// ...

type CreateRoomAndWaitOutput struct {
	Result    string           `json:"result"`
	RoomID    string           `json:"room_id"`
	Messages  []huddle.Message `json:"messages"`
	LastMsgID int64            `json:"last_msg_id"`
}

type PostMessageAndWaitInput struct {
	RoomID     string `json:"room_id" jsonschema:"ID of the room"`
	Sender     string `json:"sender" jsonschema:"Name of the sender"`
	Content    string `json:"content" jsonschema:"Message content"`
	Recipient  string `json:"recipient,omitempty" jsonschema:"Recipient name (optional)"`
	LastSeenID int64  `json:"last_seen_id" jsonschema:"ID of the last message seen by the sender"`
	TimeoutSec int    `json:"timeout_sec" jsonschema:"Optional, this service is based on the long polling mechanism, and can accept a longer timeout(default 600s) no need to modify it unless necessary"`
	Force      bool   `json:"force,omitempty" jsonschema:"If true, force post even if new messages exist since last_seen_id"`
}

type PostMessageAndWaitOutput struct {
	Result          string           `json:"result"`
	Messages        []huddle.Message `json:"messages,omitempty"`
	PreExistingMsgs []huddle.Message `json:"pre_existing_msgs,omitempty"`
	HadPreExisting  bool             `json:"had_pre_existing"`
	LastMsgID       int64            `json:"last_msg_id"`
}

type WaitForMessageInput struct {
	RoomID     string `json:"room_id" jsonschema:"ID of the room"`
	MemberName string `json:"member_name" jsonschema:"Name of the waiting member"`
	LastMsgID  int64  `json:"last_msg_id" jsonschema:"ID of the last message received"`
	TimeoutSec int    `json:"timeout_sec" jsonschema:"Optional, this service is based on the long polling mechanism, and can accept a longer timeout(default 600s), no need to modify it unless necessary"`
}

type WaitForMessageOutput struct {
	Result   string           `json:"result,omitempty"`
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

type GetRoomContextInput struct {
	RoomID     string `json:"room_id" jsonschema:"ID of the room"`
	MemberName string `json:"member_name" jsonschema:"Name of the member requesting context"`
	LastMsgID  int    `json:"last_msg_id,omitempty" jsonschema:"Start fetching messages after this ID (default 0)"`
}

type GetRoomContextOutput struct {
	Messages []huddle.Message `json:"messages"`
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "create_room_and_wait", Description: "Create a room, optionally post init message, and wait for reply"}, s.handleCreateRoomAndWait)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "post_message_and_wait", Description: "Post a message and wait for new messages. Set force=true to post regardless of new messages."}, s.handlePostMessageAndWait)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "force_post_message_and_wait", Description: "Force post a message and wait. If new messages exist since last_seen_id, returns them immediately (excluding the just-posted message) without waiting."}, s.handleForcePostMessageAndWait)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "wait_for_message", Description: "Wait for new messages in the room"}, s.handleWaitForMessage)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "get_room_context", Description: "Get message history from a specific ID (non-blocking). Use before joining discussion."}, s.handleGetRoomContext)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "list_rooms", Description: "List active meeting rooms"}, s.handleListRooms)
	mcp.AddTool(s.mcpSrv, &mcp.Tool{Name: "close_room", Description: "Close a meeting room"}, s.handleCloseRoom)
}

func (s *Server) handleGetRoomContext(ctx context.Context, req *mcp.CallToolRequest, input GetRoomContextInput) (*mcp.CallToolResult, GetRoomContextOutput, error) {
	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, GetRoomContextOutput{}, err
	}

	room.EnsureMember(input.MemberName)

	// Use 0 timeout for non-blocking fetch
	msgs, err := room.WaitForMessage(input.MemberName, int64(input.LastMsgID), 0)
	if err != nil {
		return nil, GetRoomContextOutput{}, err
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}
	return nil, GetRoomContextOutput{Messages: msgs}, nil
}

func (s *Server) handleCreateRoomAndWait(ctx context.Context, req *mcp.CallToolRequest, input CreateRoomAndWaitInput) (*mcp.CallToolResult, CreateRoomAndWaitOutput, error) {
	room, err := s.manager.CreateRoom(input.RoomID, input.Name, input.Host)
	if err != nil {
		if err == huddle.ErrRoomAlreadyExists {
			// Idempotency: Try to get the existing room
			existingRoom, getErr := s.manager.GetRoom(input.RoomID)
			if getErr != nil {
				return nil, CreateRoomAndWaitOutput{
					Result:    fmt.Sprintf("Room ID conflict: %s already exists and could not be retrieved.", input.RoomID),
					Messages:  []huddle.Message{},
					LastMsgID: 0,
				}, nil
			}
			room = existingRoom
		} else {
			return nil, CreateRoomAndWaitOutput{}, err
		}
	}

	lastMsgID := int64(0)
	if input.InitMessage != "" {
		// Check for duplicates if room already existed (or just to be safe)
		room.EnsureMember(input.Host) // Ensure member exists before checking/posting

		// Get latest state to check for duplicates and get correct LastMsgID
		// We use a short timeout to get current state without blocking long
		msgs, _ := room.WaitForMessage(input.Host, 0, 0)

		isDuplicate := false
		currentLastID := int64(0)
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			currentLastID = lastMsg.ID
			// Check if the very last message is ours and same content
			if lastMsg.Sender == input.Host && lastMsg.Content == input.InitMessage {
				isDuplicate = true
				lastMsgID = lastMsg.ID
			}
		}

		if !isDuplicate {
			// Try to post with the currentLastID we found
			id, _, err := room.PostMessage(input.Host, input.InitMessage, "", currentLastID, false)
			if err != nil {
				if err == huddle.ErrContextChanged {
					// Race condition: someone posted while we were checking.
					// Retry once.
					msgs, _ := room.WaitForMessage(input.Host, 0, 0)
					if len(msgs) > 0 {
						currentLastID = msgs[len(msgs)-1].ID
					}
					// Retry post
					id, _, err = room.PostMessage(input.Host, input.InitMessage, "", currentLastID, false)
					if err != nil {
						return nil, CreateRoomAndWaitOutput{}, fmt.Errorf("failed to post init message after retry: %v", err)
					}
					lastMsgID = id
				} else {
					return nil, CreateRoomAndWaitOutput{}, err
				}
			} else {
				lastMsgID = id
			}
		}
	}

	timeout := time.Duration(input.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = DefaultTimeoutSec * time.Second
	}

	msgs, err := room.WaitForMessage(input.Host, lastMsgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return nil, CreateRoomAndWaitOutput{
				Result:    "Room closed",
				RoomID:    room.ID,
				Messages:  []huddle.Message{},
				LastMsgID: lastMsgID,
			}, nil
		}
		return nil, CreateRoomAndWaitOutput{}, err
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	finalLastMsgID := lastMsgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return nil, CreateRoomAndWaitOutput{
		Result:    fmt.Sprintf("Room created (or joined) and waited. ID: %s", room.ID),
		RoomID:    room.ID,
		Messages:  msgs,
		LastMsgID: finalLastMsgID,
	}, nil
}

func (s *Server) handlePostMessageAndWait(ctx context.Context, req *mcp.CallToolRequest, input PostMessageAndWaitInput) (*mcp.CallToolResult, PostMessageAndWaitOutput, error) {
	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, PostMessageAndWaitOutput{}, err
	}

	room.EnsureMember(input.Sender)

	msgID, preExistingMsgs, err := room.PostMessage(input.Sender, input.Content, input.Recipient, input.LastSeenID, input.Force)
	if err != nil {
		if err == huddle.ErrContextChanged {
			// Non-force mode: context changed, return pre-existing messages for review
			return nil, PostMessageAndWaitOutput{
				Result:          "Post rejected: New messages available. Please review and retry.",
				PreExistingMsgs: preExistingMsgs,
				HadPreExisting:  true,
			}, nil
		}
		if err == huddle.ErrRoomClosed {
			return nil, PostMessageAndWaitOutput{
				Result:   "Room closed",
				Messages: []huddle.Message{},
			}, nil
		}
		return nil, PostMessageAndWaitOutput{}, err
	}

	// Force mode with pre-existing messages: return immediately
	if input.Force && len(preExistingMsgs) > 0 {
		return nil, PostMessageAndWaitOutput{
			Result:          "Message posted. Pre-existing messages found since last_seen_id.",
			PreExistingMsgs: preExistingMsgs,
			HadPreExisting:  true,
			LastMsgID:       msgID,
		}, nil
	}

	// Wait for new messages after our post
	timeout := time.Duration(input.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = DefaultTimeoutSec * time.Second
	}

	msgs, err := room.WaitForMessage(input.Sender, msgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return nil, PostMessageAndWaitOutput{
				Result:         "Room closed",
				Messages:       []huddle.Message{},
				HadPreExisting: false,
				LastMsgID:      msgID,
			}, nil
		}
		return nil, PostMessageAndWaitOutput{}, err
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	finalLastMsgID := msgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return nil, PostMessageAndWaitOutput{
		Result:         "Message posted and waited",
		Messages:       msgs,
		HadPreExisting: false,
		LastMsgID:      finalLastMsgID,
	}, nil
}

func (s *Server) handleWaitForMessage(ctx context.Context, req *mcp.CallToolRequest, input WaitForMessageInput) (*mcp.CallToolResult, WaitForMessageOutput, error) {
	timeout := time.Duration(input.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = DefaultTimeoutSec * time.Second
	}

	room, err := s.manager.GetRoom(input.RoomID)
	if err != nil {
		return nil, WaitForMessageOutput{}, err
	}

	room.EnsureMember(input.MemberName)

	msgs, err := room.WaitForMessage(input.MemberName, input.LastMsgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return nil, WaitForMessageOutput{
				Result:   "Room closed",
				Messages: []huddle.Message{},
			}, nil
		}
		return nil, WaitForMessageOutput{}, err
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	return nil, WaitForMessageOutput{Messages: msgs}, nil
}

func (s *Server) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, input ListRoomsInput) (*mcp.CallToolResult, ListRoomsOutput, error) {
	rooms := s.manager.ListRooms()
	if rooms == nil {
		rooms = []*huddle.Room{}
	}
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

// handleForcePostMessageAndWait is a wrapper that calls handlePostMessageAndWait with Force=true
func (s *Server) handleForcePostMessageAndWait(ctx context.Context, req *mcp.CallToolRequest, input PostMessageAndWaitInput) (*mcp.CallToolResult, PostMessageAndWaitOutput, error) {
	input.Force = true
	return s.handlePostMessageAndWait(ctx, req, input)
}
