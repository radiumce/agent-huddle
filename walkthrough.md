# Agent Huddle MCP Server Walkthrough

This document describes the implementation of the `agent-huddle` MCP server.

## Overview

The `agent-huddle` server implements a meeting room system for multi-agent collaboration. It supports:
- Creating and joining rooms.
- Real-time messaging with vector clock ordering.
- Host priority for concurrent messages.
- Blocking wait for new messages (long-polling style).

## Implementation Details

### Core Logic (`pkg/huddle`)
- **Room**: Manages members and messages. Uses `sync.RWMutex` for thread safety and a channel-based broadcast mechanism for blocking waits.
- **Optimistic Concurrency**: `PostMessage` requires `last_seen_id` to detect context changes.
- **Manager**: Handles room lifecycle and cleanup of idle rooms (30 min timeout).

### MCP Server (`pkg/mcp`)
- Implements standard MCP tools:
    - `create_room`, `join_room`, `list_rooms`, `close_room`
    - `post_message`, `wait_for_message`
- **Transport**: Uses **HTTP Streamable Transport** as requested.
    - The server listens on the configured address (default `:8080`) and handles MCP requests via HTTP.
    - This replaces the Stdio transport.

## How to Run

1.  **Build**:
    ```bash
    go build -o server ./cmd/server
    ```

2.  **Run**:
    ```bash
    ./server -addr :8080
    ```
    The server will listen on port 8080. Configure your MCP client to connect to this URL (e.g., `http://localhost:8080/mcp` or just the root depending on client expectation, here it serves on root).

## Usage Example (MCP Tool Call)

1.  **Create Room (with init message)**:
    ```json
    {
      "name": "create_room",
      "arguments": {
        "name": "Design Review",
        "host": "AgentA",
        "init_message": "Welcome everyone"
      }
    }
    ```

2.  **Post Message and Wait**:
    ```json
    {
      "name": "post_message_and_wait",
      "arguments": {
        "room_id": "room-123...",
        "sender": "AgentA",
        "content": "Any questions?",
        "last_seen_id": 1,
        "timeout_sec": 30
      }
    }
    ```

3.  **Create Room and Wait**:
    ```json
    {
      "name": "create_room_and_wait",
      "arguments": {
        "name": "Quick Sync",
        "host": "AgentA",
        "init_message": "Status update?",
        "timeout_sec": 60
      }
    }
    ```

4.  **Wait for Message (Implicit Join)**:
    ```json
    {
      "name": "wait_for_message",
      "arguments": {
        "room_id": "room-123...",
        "member_name": "AgentB",
        "last_msg_id": 0
      }
    }
    ```
