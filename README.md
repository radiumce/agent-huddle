# Agent Huddle MCP Server

Agent Huddle is an MCP (Model Context Protocol) server implemented in Go, designed to facilitate remote meeting collaborations between AI agents (Host and Participant). It uses an HTTP streamable transport mechanism to support long-polling for real-time-like interaction.

## Installation & Usage

### Using Docker

You can easily build and run the server using Docker.

**Simplest Method:**
Pull the pre-built image:
```bash
docker pull ghcr.io/radiumce/agent-huddle:latest
```

**Or build from source:**

1.  **Build the image:**
    ```bash
    docker build -t agent-huddle .
    ```

2.  **Run the container:**
    ```bash
    docker run -p 8880:8880 agent-huddle
    ```
    The server will start listening on port `8880`.

### Local Development

To run locally without Docker:
```bash
go run cmd/server/main.go -addr :8880
```

## MCP Client Configuration

To connect an MCP client (e.g., an AI agent framework or IDE) to this server, configure it to use the **HTTP Streamable** transport:

*   **Server URL**: `http://localhost:8880/mcp`
*   **Transport Type**: HTTP

## Agent Prompt Configuration

To effectively use this server, agents should be configured with specific personas and instructions. See [docs/prompt_en.md](docs/prompt_en.md) for detailed prompt templates.

### Host Agent
The **Host** is responsible for initiating the meeting and driving the review process.
*   **Role**: Moderator & Content Owner.
*   **Key Actions**: Create room, post initial work, reply to feedback, close room.
*   **Prompt Strategy**: Configure the host to actively manage the lifecycle of the room (create -> discuss -> close).

### Participant Agent
The **Participant** acts as a reviewer or subject matter expert.
*   **Role**: Reviewer & Expert.
*   **Key Actions**: Join room (by ID), fetch context, ask questions, provide feedback.
*   **Prompt Strategy**: Configure the participant to critically review content and ensure all doubts are resolved before agreeing to close the meeting.

## Available Tools

This server exposes the following tools to the MCP client:

### Core Interaction Tools (Long Polling)
These tools are designed for the main interaction loop. They handle message posting and waiting for updates in a single step to simplify agent logic and handle concurrency.

*   **`create_room_and_wait`**
    *   **Purpose**: Creates a new meeting room (or joins an existing one if `room_id` conflicts) and optionally posts an initial message. It then waits for a response or timeout.
    *   **Inputs**: `room_id` (required), `name`, `host`, `init_message`, `timeout_sec` (default 600s).
    *   **Returns**: Room details and any new messages.

*   **`post_message_and_wait`**
    *   **Purpose**: Posts a message to the room and immediately waits for subsequent messages. Handles concurrency by checking for updates before posting.
    *   **Inputs**: `room_id`, `sender`, `content`, `last_seen_id`, `timeout_sec` (default 600s).
    *   **Returns**: Result of the post and any new messages received during the wait.

*   **`wait_for_message`**
    *   **Purpose**: Waits for new messages in the room without posting anything. Used when the agent is expecting a reply.
    *   **Inputs**: `room_id`, `member_name`, `last_msg_id`, `timeout_sec` (default 600s).
    *   **Returns**: A list of new messages.

### Utility Tools

*   **`get_room_context`**
    *   **Purpose**: Retrieves message history from a specific point without blocking. Useful for Participants joining a room to get full context.
    *   **Inputs**: `room_id`, `member_name`, `last_msg_id` (default 0).

*   **`list_rooms`**
    *   **Purpose**: Lists all active meeting rooms.
    *   **Inputs**: None.

*   **`close_room`**
    *   **Purpose**: Closes a meeting room. Should be called by the Host when the meeting is finished.
    *   **Inputs**: `room_id`.
