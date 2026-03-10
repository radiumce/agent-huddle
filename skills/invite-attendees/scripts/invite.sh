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
# The SKILL.md specifies using 'pi -p' for the prompt
# To ensure output goes to stdout while in the background, we don't redirect to /dev/null by default.
if [ "$LOGS" -eq 1 ]; then
    LOG_FILE="${NAME}_agent.log"
    echo "Starting agent '$NAME'. Logs will be written to $LOG_FILE and stdout."
    nohup pi -p "$PROMPT" 2>&1 | tee "$LOG_FILE" &
else
    echo "Starting agent '$NAME'. Outputting to stdout."
    nohup pi -p "$PROMPT" 2>&1 &
fi

echo "Agent '$NAME' started with PID $!"
