package mcp

import (
	"context"
	"encoding/json"

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

	resp, err := s.service.GetRoomContext(ctx, roomID, memberName, lastMsgID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handleCreateRoomAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	name := req.GetString("name", "")
	host := req.GetString("host", "")
	initMessage := req.GetString("init_message", "")
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)

	resp, err := s.service.CreateRoomAndWait(ctx, roomID, name, host, initMessage, timeoutSec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handlePostMessageAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.doPostMessageAndWait(ctx, req, false)
}

func (s *Server) handleForcePostMessageAndWait(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.doPostMessageAndWait(ctx, req, true)
}

func (s *Server) doPostMessageAndWait(ctx context.Context, req mcp.CallToolRequest, forceOverride bool) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	sender := req.GetString("sender", "")
	content := req.GetString("content", "")
	recipient := req.GetString("recipient", "")
	lastSeenID := int64(req.GetInt("last_seen_id", 0))
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)
	force := req.GetBool("force", false) || forceOverride

	resp, err := s.service.PostMessageAndWait(ctx, roomID, sender, content, recipient, lastSeenID, timeoutSec, force)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handleWaitForMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	memberName := req.GetString("member_name", "")
	lastMsgID := int64(req.GetInt("last_msg_id", 0))
	timeoutSec := req.GetInt("timeout_sec", DefaultTimeoutSec)

	resp, err := s.service.WaitForMessage(ctx, roomID, memberName, lastMsgID, timeoutSec)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handleListRooms(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp, err := s.service.ListRooms(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handleCloseRoom(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")

	resp, err := s.service.CloseRoom(ctx, roomID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}

func (s *Server) handleLeaveRoom(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := req.GetString("room_id", "")
	memberName := req.GetString("member_name", "")

	resp, err := s.service.LeaveRoom(ctx, roomID, memberName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(resp)
}
