#!/bin/bash

# invite.sh
# Starts an agent invitation in a detached screen session.
# Usage: ./invite.sh -n <session_name> -p <prompt>

usage() {
    echo "Usage: $0 -n <name> -p <prompt>"
    exit 1
}

NAME=""
PROMPT=""

# Parse arguments
while getopts "n:p:" opt; do
    case $opt in
        n) NAME="$OPTARG" ;;
        p) PROMPT="$OPTARG" ;;
        *) usage ;;
    esac
done

if [ -z "$NAME" ] || [ -z "$PROMPT" ]; then
    usage
fi

# Start the agent in a detached screen session
# The SKILL.md specifies using 'goose run --text'
screen -dmS "$NAME" goose run --text "$PROMPT"
