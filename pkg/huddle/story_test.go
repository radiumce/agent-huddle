package huddle

import (
	"sync"
	"testing"
	"time"
)

// ST-1: 发起review会议
// * Host creates room
// * Host posts first message
// * Participant joins and gets history
func TestStory1_CreateAndJoin(t *testing.T) {
	mgr := NewManager()
	
	// Host creates room
	room, err := mgr.CreateRoom("", "Project Alpha", "Alice")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	
	if room.Name != "Project Alpha" {
		t.Errorf("Expected room name 'Project Alpha', got %s", room.Name)
	}
	
	// Host posts first message
	_, _, err = room.PostMessage("HostAgent", "Welcome to the review", "", 0, false)
	if err != nil {
		t.Fatalf("Failed to post message: %v", err)
	}
	
	// Participant joins implicitly via EnsureMember (simulating tool behavior)
	room.EnsureMember("ParticipantAgent")
	
	// Participant gets history
	msgs, err := room.WaitForMessage("ParticipantAgent", 0, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(msgs) != 1 {
		t.Errorf("Expected 1 message in history, got %d", len(msgs))
	}
	if msgs[0].Content != "Welcome to the review" {
		t.Errorf("Unexpected message content: %s", msgs[0].Content)
	}
}

// ST-2: 参与会议讨论
// * Blocking wait
// * Host priority (concurrent messages)
func TestStory2_Discussion(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("", "Discussion Room", "Alice")
	
	// Add participant implicitly
	room.EnsureMember("Participant")
	
	// Test Blocking Wait
	var receivedMsg []Message
	var waitErr error
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		// Wait for message after ID 0
		receivedMsg, waitErr = room.WaitForMessage("Participant", 0, 2*time.Second)
	}()
	
	time.Sleep(50 * time.Millisecond)
	
	// Host posts a message
	room.PostMessage("Host", "Question 1", "", 0, false)
	
	<-done
	if waitErr != nil {
		t.Fatalf("WaitForMessage failed: %v", waitErr)
	}
	if len(receivedMsg) != 1 || receivedMsg[0].Content != "Question 1" {
		t.Errorf("Expected 'Question 1', got %v", receivedMsg)
	}
	
	// Test Optimistic Concurrency (Context Update)
	// Current ID is 1. Host posts message 2.
	room.PostMessage("Host", "Answer 1", "", 1, false)
	
	// Participant tries to post relying on ID 1 (which is now old)
	_, _, err := room.PostMessage("Participant", "My Comment", "", 1, false)
	if err != ErrContextChanged {
		t.Errorf("Expected ErrContextChanged, got %v", err)
	}
	
	// Simulate Tool Handler Logic: Fetch missed messages
	if err == ErrContextChanged {
		msgs, fetchErr := room.WaitForMessage("Participant", 1, 0)
		if fetchErr != nil {
			t.Fatalf("Failed to fetch missed messages: %v", fetchErr)
		}
		if len(msgs) != 1 || msgs[0].Content != "Answer 1" {
			t.Errorf("Expected to fetch 'Answer 1', got %v", msgs)
		}
	}
	
	// Now Participant posts with correct ID (2)
	_, _, err = room.PostMessage("Participant", "My Comment", "", 2, false)
	if err != nil {
		t.Errorf("Failed to post with correct ID: %v", err)
	}
}

// ST-3: 结束会议
func TestStory3_Close(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("", "Closing Room", "Alice")
	
	// Close room
	room.Close()
	
	// Try to post
	_, _, err := room.PostMessage("Host", "Late message", "", 0, false)
	if err != ErrRoomClosed {
		t.Errorf("Expected ErrRoomClosed, got %v", err)
	}
	
	// Try to wait
	_, err = room.WaitForMessage("Host", 0, 1*time.Second)
	if err != ErrRoomClosed {
		t.Errorf("Expected ErrRoomClosed on wait, got %v", err)
	}
}

// Concurrent Stress Test (Bonus)
func TestConcurrentMessaging(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("", "Stress Room", "Host")
	
	var wg sync.WaitGroup
	// 10 participants posting 10 messages each
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "P" // string(id)
			// Simple loop: read latest, try to post, if fail retry
			for j := 0; j < 10; j++ {
				for {
					// Get latest ID
					room.mu.RLock()
					lastID := int64(0)
					if len(room.Messages) > 0 {
						lastID = room.Messages[len(room.Messages)-1].ID
					}
					room.mu.RUnlock()
					
					// Try post
					_, _, err := room.PostMessage(name, "msg", "", lastID, false)
					if err == nil {
						break
					}
					// If err, retry (simulate fetching updates)
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	if len(room.Messages) != 100 {
		t.Errorf("Expected 100 messages, got %d", len(room.Messages))
	}
}

// Test GetRoomContext (Non-blocking history)
func TestGetRoomContext(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("", "Context Room", "Host")
	
	// Post some messages
	room.PostMessage("Host", "Msg 1", "", 0, false)
	room.PostMessage("Host", "Msg 2", "", 1, false)
	room.PostMessage("Host", "Msg 3", "", 2, false)
	
	// New participant wants context from beginning
	room.EnsureMember("NewGuy")
	
	// Fetch from 0
	msgs, err := room.WaitForMessage("NewGuy", 0, 0) // 0 timeout = non-blocking
	if err != nil {
		t.Fatalf("Failed to get context: %v", err)
	}
	
	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "Msg 1" || msgs[2].Content != "Msg 3" {
		t.Errorf("Unexpected message content in context")
	}
	
	// Fetch from ID 2
	msgs, err = room.WaitForMessage("NewGuy", 2, 0)
	if err != nil {
		t.Fatalf("Failed to get partial context: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("Expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Msg 3" {
		t.Errorf("Expected 'Msg 3', got '%s'", msgs[0].Content)
	}
}

// Test Duplicate Post (Idempotency)
// Note: This tests the logic that the MCP tool would implement.
// The Room.PostMessage itself correctly returns ErrContextChanged.
// We verify here that if we get that error, we can find our own message in the history.
func TestDuplicatePost(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("", "Dedupe Room", "Host")
	
	// 1. Host posts a message (ID 1)
	room.PostMessage("Host", "Original Message", "", 0, false)
	
	// 2. Host tries to post the SAME message again with old ID (0)
	// This simulates a retry where the client didn't see the success of the first post.
	_, _, err := room.PostMessage("Host", "Original Message", "", 0, false)
	
	if err != ErrContextChanged {
		t.Errorf("Expected ErrContextChanged for duplicate post with old ID, got %v", err)
	}
	
	// 3. Simulate Tool Logic: Fetch new messages
	if err == ErrContextChanged {
		msgs, fetchErr := room.WaitForMessage("Host", 0, 0)
		if fetchErr != nil {
			t.Fatalf("Failed to fetch messages: %v", fetchErr)
		}
		
		// 4. Check if we find our own message
		found := false
		for _, m := range msgs {
			if m.Sender == "Host" && m.Content == "Original Message" {
				found = true
				break
			}
		}
		
		if !found {
			t.Errorf("Failed to find original message in history for deduplication")
		}
	}
}

func TestCreateRoomIdempotency(t *testing.T) {
	mgr := NewManager()
	roomID := "idempotent-room"
	host := "Host"
	initMsg := "Hello World"

	// 1. First creation
	room, err := mgr.CreateRoom(roomID, "Room", host)
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	_, _, err = room.PostMessage(host, initMsg, "", 0, false)
	if err != nil {
		t.Fatalf("Failed to post init message: %v", err)
	}

	// 2. Simulate second call (Idempotency Logic)
	// This logic mirrors what we implemented in the MCP tool handler
	
	// Try create again
	_, err = mgr.CreateRoom(roomID, "Room", host)
	if err != ErrRoomAlreadyExists {
		t.Errorf("Expected ErrRoomAlreadyExists, got %v", err)
	}

	// "Tool" logic: Get existing room
	existingRoom, err := mgr.GetRoom(roomID)
	if err != nil {
		t.Fatalf("Failed to get existing room: %v", err)
	}

	// "Tool" logic: Check for duplicate init message
	existingRoom.EnsureMember(host)
	msgs, _ := existingRoom.WaitForMessage(host, 0, 0)
	
	isDuplicate := false
	if len(msgs) > 0 {
		lastMsg := msgs[len(msgs)-1]
		if lastMsg.Sender == host && lastMsg.Content == initMsg {
			isDuplicate = true
		}
	}

	if !isDuplicate {
		t.Errorf("Expected to detect duplicate message")
	}

	// Verify only 1 message exists
	if len(existingRoom.Messages) != 1 {
		t.Errorf("Expected 1 message in room, got %d", len(existingRoom.Messages))
	}
}
