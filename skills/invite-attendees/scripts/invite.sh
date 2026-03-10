#!/bin/bash

# invite.sh
# Starts an agent invitation in the background.
# Usage: ./invite.sh -n <session_name> -p <prompt> [--logs]

usage() {
    echo "Usage: $0 -n <name> -p <prompt> [--logs]"
    exit 1
}

NAME=""
PROMPT=""
LOGS=0

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -n|--name) NAME="$2"; shift ;;
        -p|--prompt) PROMPT="$2"; shift ;;
        --logs) LOGS=1 ;;
        *) usage ;;
    esac
    shift
done

if [ -z "$NAME" ] || [ -z "$PROMPT" ]; then
    usage
fi

# Start the agent in the background
# We must redirect stdout and stderr so that the parent agent/shell does not block waiting for EOF.
if [ "$LOGS" -eq 1 ]; then
    LOG_FILE="${NAME}_agent.log"
    echo "Starting agent '$NAME'. Logs will be written to $LOG_FILE."
    nohup pi -p "$PROMPT" > "$LOG_FILE" 2>&1 &
else
    echo "Starting agent '$NAME' in background."
    nohup pi -p "$PROMPT" > /dev/null 2>&1 &
fi

echo "Agent '$NAME' started with PID $!"
