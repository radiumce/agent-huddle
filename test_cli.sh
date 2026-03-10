#!/bin/bash
set -e

echo "Starting Server..."
./server -addr :8880 -api-addr :8881 > server.log 2>&1 &
SERVER_PID=$!

# Wait for server to start
sleep 2

echo "======================================"
echo "1. Creating Room"
echo "======================================"
huddle-cli create --room-id "e2e-test" --host "Alice" --init-message "Welcome to the test." --timeout 2

echo "======================================"
echo "2. Listing Rooms"
echo "======================================"
huddle-cli list

echo "======================================"
echo "3. Posting Message"
echo "======================================"
huddle-cli post --room-id "e2e-test" --sender "Bob" --content "Hello Alice, I am here." --last-id 1 --timeout 2

echo "======================================"
echo "4. Posting Message (Force)"
echo "======================================"
huddle-cli post --force --room-id "e2e-test" --sender "Charlie" --content "Sorry I'm late!" --last-id 0 --timeout 2

echo "======================================"
echo "5. Wait For Message"
echo "======================================"
# Alice waits. Since there are messages she hasn't seen (2 and 3), she should get them immediately.
huddle-cli wait --room-id "e2e-test" --member "Alice" --last-id 1 --timeout 2

echo "======================================"
echo "6. Get Context"
echo "======================================"
huddle-cli context --room-id "e2e-test" --member "Bob" --last-id 0

echo "======================================"
echo "7. Leave Room"
echo "======================================"
huddle-cli leave --room-id "e2e-test" --member "Bob"

echo "======================================"
echo "8. Get Context (After Leave)"
echo "======================================"
huddle-cli context --room-id "e2e-test" --member "Alice" --last-id 0

echo "======================================"
echo "9. Close Room"
echo "======================================"
huddle-cli close --room-id "e2e-test"

echo "======================================"
echo "10. Listing Rooms (After Close)"
echo "======================================"
huddle-cli list

echo "======================================"
echo "Stopping Server..."
kill $SERVER_PID
echo "Done."
