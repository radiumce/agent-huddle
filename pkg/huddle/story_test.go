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
	room, err := mgr.CreateRoom("Review Meeting", "HostAgent")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	
	if room.Name != "Review Meeting" {
		t.Errorf("Expected room name 'Review Meeting', got %s", room.Name)
	}
	
	// Host posts first message
	err = room.PostMessage("HostAgent", "Welcome to the review", "", 0)
	if err != nil {
		t.Fatalf("Failed to post message: %v", err)
	}
	
	// Participant joins
	member := &Member{
		Name:         "ParticipantAgent",
		IsHost:       false,
		Active:       true,
		LastActiveAt: time.Now(),
	}
	room.Join(member)
	
	// Participant gets history (wait with 0 timeout or just check messages)
	// In real flow, they might call WaitForMessage(0)
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
// * Host priority (concurrent messages) - This is hard to deterministically test without hooks, 
//   but we can test the "Context Changed" error which is the mechanism for it.
func TestStory2_Discussion(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("Discussion Room", "Host")
	
	// Add participant
	p := &Member{Name: "Participant", Active: true}
	room.Join(p)
	
	// Test Blocking Wait
	// We start a goroutine that waits for a message
	var receivedMsg []Message
	var waitErr error
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		// Wait for message after ID 0
		receivedMsg, waitErr = room.WaitForMessage("Participant", 0, 2*time.Second)
	}()
	
	// Small sleep to ensure waiter is active
	time.Sleep(50 * time.Millisecond)
	
	// Host posts a message
	room.PostMessage("Host", "Question 1", "", 0)
	
	<-done
	if waitErr != nil {
		t.Fatalf("WaitForMessage failed: %v", waitErr)
	}
	if len(receivedMsg) != 1 || receivedMsg[0].Content != "Question 1" {
		t.Errorf("Expected 'Question 1', got %v", receivedMsg)
	}
	
	// Test Optimistic Concurrency (Context Update)
	// Current ID is 1.
	// Host posts message 2.
	room.PostMessage("Host", "Answer 1", "", 1)
	
	// Participant tries to post relying on ID 1 (which is now old, because ID 2 exists)
	// Wait, if Participant saw ID 1, and Host posted ID 2.
	// Participant tries to post with LastSeenID = 1.
	// The room now has ID 2. So LastSeenID(1) < LastMsgID(2).
	// Should fail with ErrContextChanged.
	
	err := room.PostMessage("Participant", "My Comment", "", 1)
	if err != ErrContextChanged {
		t.Errorf("Expected ErrContextChanged, got %v", err)
	}
	
	// Participant should now fetch updates
	msgs, _ := room.WaitForMessage("Participant", 1, 0)
	if len(msgs) != 1 || msgs[0].Content != "Answer 1" {
		t.Errorf("Expected to fetch 'Answer 1', got %v", msgs)
	}
	
	// Now Participant posts with correct ID (2)
	err = room.PostMessage("Participant", "My Comment", "", 2)
	if err != nil {
		t.Errorf("Failed to post with correct ID: %v", err)
	}
}

// ST-3: 结束会议
func TestStory3_Close(t *testing.T) {
	mgr := NewManager()
	room, _ := mgr.CreateRoom("Closing Room", "Host")
	
	// Close room
	room.Close()
	
	// Try to post
	err := room.PostMessage("Host", "Late message", "", 0)
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
	room, _ := mgr.CreateRoom("Stress Room", "Host")
	
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
					err := room.PostMessage(name, "msg", "", lastID)
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
