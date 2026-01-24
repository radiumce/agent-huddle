package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chene/agent-huddle/pkg/huddle"
	"github.com/mark3labs/mcp-go/mcp"
)

const DefaultTimeoutSec = 300

func (s *Server) registerTools() {
	// create_room_and_wait
	s.mcpSrv.AddTool(
		mcp.NewTool("create_room_and_wait",
			mcp.WithDescription("Create a room, optionally post init message, and wait for reply"),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("Unique ID for the room")),
			mcp.WithString("name", mcp.Description("Name of the meeting room")),
			mcp.WithString("host", mcp.Required(), mcp.Description("Name of the host agent")),
			mcp.WithString("init_message", mcp.Description("Optional initial message to post")),
			mcp.WithNumber("timeout_sec", mcp.Description("Optional timeout in seconds (default 300s)")),
		),
		s.handleCreateRoomAndWait,
	)

	// post_message_and_wait
	s.mcpSrv.AddTool(
		mcp.NewTool("post_message_and_wait",
			mcp.WithDescription("Post a message and wait for new messages. Set force=true to post regardless of new messages."),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
			mcp.WithString("sender", mcp.Required(), mcp.Description("Name of the sender")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Message content")),
			mcp.WithString("recipient", mcp.Description("Recipient name (optional)")),
			mcp.WithNumber("last_seen_id", mcp.Required(), mcp.Description("ID of the last message seen by the sender")),
			mcp.WithNumber("timeout_sec", mcp.Description("Optional timeout in seconds (default 300s)")),
			mcp.WithBoolean("force", mcp.Description("If true, force post even if new messages exist since last_seen_id")),
		),
		s.handlePostMessageAndWait,
	)

	// force_post_message_and_wait
	s.mcpSrv.AddTool(
		mcp.NewTool("force_post_message_and_wait",
			mcp.WithDescription("Force post a message and wait. If new messages exist since last_seen_id, returns them immediately (excluding the just-posted message) without waiting."),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
			mcp.WithString("sender", mcp.Required(), mcp.Description("Name of the sender")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Message content")),
			mcp.WithString("recipient", mcp.Description("Recipient name (optional)")),
			mcp.WithNumber("last_seen_id", mcp.Required(), mcp.Description("ID of the last message seen by the sender")),
			mcp.WithNumber("timeout_sec", mcp.Description("Optional timeout in seconds (default 300s)")),
		),
		s.handleForcePostMessageAndWait,
	)

	// wait_for_message
	s.mcpSrv.AddTool(
		mcp.NewTool("wait_for_message",
			mcp.WithDescription("Wait for new messages in the room"),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
			mcp.WithString("member_name", mcp.Required(), mcp.Description("Name of the waiting member")),
			mcp.WithNumber("last_msg_id", mcp.Required(), mcp.Description("ID of the last message received")),
			mcp.WithNumber("timeout_sec", mcp.Description("Optional timeout in seconds (default 300s)")),
		),
		s.handleWaitForMessage,
	)

	// get_room_context
	s.mcpSrv.AddTool(
		mcp.NewTool("get_room_context",
			mcp.WithDescription("Get message history from a specific ID (non-blocking). Use before joining discussion."),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
			mcp.WithString("member_name", mcp.Required(), mcp.Description("Name of the member requesting context")),
			mcp.WithNumber("last_msg_id", mcp.Description("Start fetching messages after this ID (default 0)")),
		),
		s.handleGetRoomContext,
	)

	// list_rooms
	s.mcpSrv.AddTool(
		mcp.NewTool("list_rooms",
			mcp.WithDescription("List active meeting rooms"),
		),
		s.handleListRooms,
	)

	// close_room
	s.mcpSrv.AddTool(
		mcp.NewTool("close_room",
			mcp.WithDescription("Close a meeting room"),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
		),
		s.handleCloseRoom,
	)

	// leave_room
	s.mcpSrv.AddTool(
		mcp.NewTool("leave_room",
			mcp.WithDescription("Leave a meeting room. This notifies other participants that you have left."),
			mcp.WithString("room_id", mcp.Required(), mcp.Description("ID of the room")),
			mcp.WithString("member_name", mcp.Required(), mcp.Description("Name of the member leaving")),
		),
		s.handleLeaveRoom,
	)
}

// Helper function to return JSON result
func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGetRoomContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	memberName := req.GetString("member_name", "")
	lastMsgID := int64(req.GetInt("last_msg_id", 0))

	room, err := s.manager.GetRoom(roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	room.EnsureMember(memberName)

	// Use 0 timeout for non-blocking fetch
	msgs, err := room.WaitForMessage(memberName, lastMsgID, 0)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}
	return jsonResult(map[string]interface{}{"messages": msgs})
}

func (s *Server) handleCreateRoomAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	name := req.GetString("name", "")
	host := req.GetString("host", "")
	initMessage := req.GetString("init_message", "")
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)

	room, err := s.manager.CreateRoom(roomID, name, host)
	if err != nil {
		if err == huddle.ErrRoomAlreadyExists {
			// Idempotency: Try to get the existing room
			existingRoom, getErr := s.manager.GetRoom(roomID)
			if getErr != nil {
				return jsonResult(map[string]interface{}{
					"result":      fmt.Sprintf("Room ID conflict: %s already exists and could not be retrieved.", roomID),
					"messages":    []huddle.Message{},
					"last_msg_id": 0,
				})
			}
			room = existingRoom
		} else {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	lastMsgID := int64(0)
	if initMessage != "" {
		// Check for duplicates if room already existed (or just to be safe)
		room.EnsureMember(host) // Ensure member exists before checking/posting

		// Get latest state to check for duplicates and get correct LastMsgID
		msgs, _ := room.WaitForMessage(host, 0, 0)

		isDuplicate := false
		currentLastID := int64(0)
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			currentLastID = lastMsg.ID
			// Check if the very last message is ours and same content
			if lastMsg.Sender == host && lastMsg.Content == initMessage {
				isDuplicate = true
				lastMsgID = lastMsg.ID
			}
		}

		if !isDuplicate {
			// Try to post with the currentLastID we found
			id, _, err := room.PostMessage(host, initMessage, "", currentLastID, false)
			if err != nil {
				if err == huddle.ErrContextChanged {
					// Race condition: someone posted while we were checking.
					// Retry once.
					msgs, _ := room.WaitForMessage(host, 0, 0)
					if len(msgs) > 0 {
						currentLastID = msgs[len(msgs)-1].ID
					}
					// Retry post
					id, _, err = room.PostMessage(host, initMessage, "", currentLastID, false)
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("failed to post init message after retry: %v", err)), nil
					}
					lastMsgID = id
				} else {
					return mcp.NewToolResultError(err.Error()), nil
				}
			} else {
				lastMsgID = id
			}
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	msgs, err := room.WaitForMessage(host, lastMsgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return jsonResult(map[string]interface{}{
				"result":      "Room closed",
				"room_id":     room.ID,
				"messages":    []huddle.Message{},
				"last_msg_id": lastMsgID,
			})
		}
		return mcp.NewToolResultError(err.Error()), nil
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	finalLastMsgID := lastMsgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return jsonResult(map[string]interface{}{
		"result":      fmt.Sprintf("Room created (or joined) and waited. ID: %s", room.ID),
		"room_id":     room.ID,
		"messages":    msgs,
		"last_msg_id": finalLastMsgID,
	})
}

func (s *Server) handlePostMessageAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.doPostMessageAndWait(ctx, req, false)
}

func (s *Server) handleForcePostMessageAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.doPostMessageAndWait(ctx, req, true)
}

func (s *Server) doPostMessageAndWait(_ context.Context, req mcp.CallToolRequest, forceOverride bool) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	sender := req.GetString("sender", "")
	content := req.GetString("content", "")
	recipient := req.GetString("recipient", "")
	lastSeenID := int64(req.GetInt("last_seen_id", 0))
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)
	force := req.GetBool("force", false) || forceOverride

	room, err := s.manager.GetRoom(roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	room.EnsureMember(sender)

	msgID, preExistingMsgs, err := room.PostMessage(sender, content, recipient, lastSeenID, force)
	if err != nil {
		if err == huddle.ErrContextChanged {
			// Non-force mode: context changed, return pre-existing messages for review
			return jsonResult(map[string]interface{}{
				"result":            "Post rejected: New messages available. Please review and retry.",
				"pre_existing_msgs": preExistingMsgs,
				"had_pre_existing":  true,
			})
		}
		if err == huddle.ErrRoomClosed {
			return jsonResult(map[string]interface{}{
				"result":   "Room closed",
				"messages": []huddle.Message{},
			})
		}
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Force mode with pre-existing messages: return immediately
	if force && len(preExistingMsgs) > 0 {
		return jsonResult(map[string]interface{}{
			"result":            "Message posted. Pre-existing messages found since last_seen_id.",
			"pre_existing_msgs": preExistingMsgs,
			"had_pre_existing":  true,
			"last_msg_id":       msgID,
		})
	}

	// Wait for new messages after our post
	timeout := time.Duration(timeoutSec) * time.Second

	msgs, err := room.WaitForMessage(sender, msgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return jsonResult(map[string]interface{}{
				"result":           "Room closed",
				"messages":         []huddle.Message{},
				"had_pre_existing": false,
				"last_msg_id":      msgID,
			})
		}
		return mcp.NewToolResultError(err.Error()), nil
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	finalLastMsgID := msgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return jsonResult(map[string]interface{}{
		"result":           "Message posted and waited",
		"messages":         msgs,
		"had_pre_existing": false,
		"last_msg_id":      finalLastMsgID,
	})
}

func (s *Server) handleWaitForMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	memberName := req.GetString("member_name", "")
	lastMsgID := int64(req.GetInt("last_msg_id", 0))
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)

	timeout := time.Duration(timeoutSec) * time.Second

	room, err := s.manager.GetRoom(roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	room.EnsureMember(memberName)

	msgs, err := room.WaitForMessage(memberName, lastMsgID, timeout)
	if err != nil {
		if err == huddle.ErrRoomClosed {
			return jsonResult(map[string]interface{}{
				"result":   "Room closed",
				"messages": []huddle.Message{},
			})
		}
		return mcp.NewToolResultError(err.Error()), nil
	}

	if msgs == nil {
		msgs = []huddle.Message{}
	}

	return jsonResult(map[string]interface{}{"messages": msgs})
}

func (s *Server) handleListRooms(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rooms := s.manager.ListRooms()
	if rooms == nil {
		rooms = []*huddle.Room{}
	}
	return jsonResult(map[string]interface{}{"rooms": rooms})
}

func (s *Server) handleCloseRoom(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")

	room, err := s.manager.GetRoom(roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	room.Close()
	return jsonResult(map[string]interface{}{"result": "Room closed"})
}

func (s *Server) handleLeaveRoom(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	memberName := req.GetString("member_name", "")

	room, err := s.manager.GetRoom(roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = room.LeaveRoom(memberName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]interface{}{
		"result": fmt.Sprintf("Member '%s' has left the room", memberName),
	})
}
