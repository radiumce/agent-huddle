package huddle

import (
	"sync"
	"testing"
	"time"
)

// TestLeaveRoom tests the leave room functionality
func TestLeaveRoom(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("leave-test", "Leave Test Room", "Host")

	// Add a participant
	room.EnsureMember("Participant1")

	// Verify member exists and is active
	member := room.Members["Participant1"]
	if member.Left {
		t.Error("Member should not be marked as left initially")
	}

	// Leave the room
	err := room.LeaveRoom("Participant1")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	// Verify member is now marked as left
	if !member.Left {
		t.Error("Member should be marked as left after LeaveRoom")
	}
	if member.Active {
		t.Error("Member should be inactive after LeaveRoom")
	}

	// Verify system message was posted
	if len(room.Messages) != 1 {
		t.Errorf("Expected 1 system message, got %d", len(room.Messages))
	}
	if room.Messages[0].Sender != "[System]" {
		t.Errorf("Expected sender '[System]', got '%s'", room.Messages[0].Sender)
	}
}

// TestHeartbeat tests the heartbeat notification mechanism
func TestHeartbeat(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("heartbeat-test", "Heartbeat Test Room", "Host")

	// Reduce heartbeat interval for faster testing
	room.HeartbeatInterval = 100 * time.Millisecond

	// Add participants (need >= 2 for heartbeat)
	room.EnsureMember("Host")
	room.EnsureMember("Participant1")

	var wg sync.WaitGroup
	wg.Add(2)

	// Both participants start waiting concurrently
	go func() {
		defer wg.Done()
		room.WaitForMessage("Host", 0, 2*time.Second)
	}()

	go func() {
		defer wg.Done()
		room.WaitForMessage("Participant1", 0, 2*time.Second)
	}()

	// Give time for both waits to start
	time.Sleep(200 * time.Millisecond)

	// Manually trigger heartbeat check (simulating ticker fire)
	room.checkAndSendHeartbeat()

	wg.Wait()

	// Check that heartbeat message was sent
	room.mu.RLock()
	msgCount := len(room.Messages)
	hasHeartbeat := false
	for _, msg := range room.Messages {
		if msg.Sender == "[System]" && msg.Content == room.HeartbeatMessage {
			hasHeartbeat = true
			break
		}
	}
	room.mu.RUnlock()

	if msgCount == 0 {
		t.Error("Expected at least one message (heartbeat)")
	}
	if !hasHeartbeat {
		t.Error("Expected heartbeat message to be broadcast")
	}
}

// TestHeartbeatNotTriggeredWithSingleMember tests that heartbeat is NOT triggered with only 1 member
func TestHeartbeatNotTriggeredWithSingleMember(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("single-heartbeat-test", "Single Heartbeat Test Room", "Host")

	// Reduce heartbeat interval for faster testing
	room.HeartbeatInterval = 500 * time.Millisecond

	// Only one member (Host)
	room.EnsureMember("Host")

	// Host starts waiting
	msgs, _ := room.WaitForMessage("Host", 0, 2*time.Second)

	// Should timeout with no messages (no heartbeat for single member)
	if len(msgs) > 0 {
		t.Errorf("Expected no heartbeat for single member, got %d messages", len(msgs))
	}

	// Double-check room messages
	room.mu.RLock()
	msgCount := len(room.Messages)
	room.mu.RUnlock()

	if msgCount > 0 {
		t.Errorf("Expected 0 messages in room (no heartbeat for single member), got %d", msgCount)
	}
}

// TestHeartbeatWithLeftMember tests that left members don't count toward heartbeat
func TestHeartbeatWithLeftMember(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("left-heartbeat-test", "Left Heartbeat Test Room", "Host")

	// Reduce heartbeat interval for faster testing
	room.HeartbeatInterval = 500 * time.Millisecond

	// Add two members
	room.EnsureMember("Host")
	room.EnsureMember("Participant1")

	// Participant1 leaves
	room.LeaveRoom("Participant1")

	// Clear the leave message for cleaner test
	room.mu.Lock()
	room.Messages = room.Messages[:0]
	room.mu.Unlock()

	// Only Host is active now, so no heartbeat should trigger
	msgs, _ := room.WaitForMessage("Host", 0, 2*time.Second)

	// Should timeout with no heartbeat (only 1 active member)
	room.mu.RLock()
	heartbeatCount := 0
	for _, msg := range room.Messages {
		if msg.Sender == "[System]" && msg.Content == room.HeartbeatMessage {
			heartbeatCount++
		}
	}
	room.mu.RUnlock()

	if heartbeatCount > 0 {
		t.Errorf("Expected no heartbeat (only 1 active member after leave), got %d", heartbeatCount)
	}

	_ = msgs // suppress unused variable warning
}

// TestWaitingSinceTracking tests that WaitingSince is correctly set and cleared
func TestWaitingSinceTracking(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("waiting-track-test", "Waiting Track Test Room", "Host")

	room.EnsureMember("Host")

	// Before wait: WaitingSince should be nil
	room.mu.RLock()
	if room.Members["Host"].WaitingSince != nil {
		t.Error("WaitingSince should be nil before waiting")
	}
	room.mu.RUnlock()

	// Start waiting in goroutine
	done := make(chan struct{})
	go func() {
		room.WaitForMessage("Host", 0, 500*time.Millisecond)
		close(done)
	}()

	// Give time for wait to start
	time.Sleep(100 * time.Millisecond)

	// During wait: WaitingSince should be set
	room.mu.RLock()
	if room.Members["Host"].WaitingSince == nil {
		t.Error("WaitingSince should be set during waiting")
	}
	room.mu.RUnlock()

	// Wait for completion
	<-done

	// After wait: WaitingSince should be cleared
	room.mu.RLock()
	if room.Members["Host"].WaitingSince != nil {
		t.Error("WaitingSince should be nil after waiting completes")
	}
	room.mu.RUnlock()
}
