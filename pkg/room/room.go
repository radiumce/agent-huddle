package room

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Role defines the role of a member in the meeting
type Role string

const (
	RoleHost        Role = "host"
	RoleParticipant Role = "participant"
)

// Message represents a chat message in the room
type Message struct {
	ID        int64     `json:"id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	// RecipientID is optional, for directed messages (though all are broadcast)
	RecipientID string `json:"recipient_id,omitempty"`
}

// Member represents a participant in the meeting
type Member struct {
	ID        string
	Role      Role
	LastMsgID int64 // The ID of the last message this member has seen/processed
}

// Room represents a meeting room
type Room struct {
	ID           string
	Name         string
	Messages     []Message
	Members      map[string]*Member
	LastActivity time.Time
	mu           sync.RWMutex
	
	// broadcast is a channel to notify waiting members of new messages
	broadcast chan struct{}
}

// NewRoom creates a new meeting room
func NewRoom(id, name string) *Room {
	return &Room{
		ID:           id,
		Name:         name,
		Messages:     make([]Message, 0),
		Members:      make(map[string]*Member),
		LastActivity: time.Now(),
		broadcast:    make(chan struct{}),
	}
}

// Join adds a member to the room
func (r *Room) Join(memberID string, role Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.LastActivity = time.Now()
	if _, exists := r.Members[memberID]; exists {
		return nil // Already joined
	}

	r.Members[memberID] = &Member{
		ID:        memberID,
		Role:      role,
		LastMsgID: 0,
	}
	return nil
}

// SendMessage adds a message to the room
func (r *Room) SendMessage(senderID, content, recipientID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.LastActivity = time.Now()
	
	// Simple auto-increment ID based on message count + 1
	// In a real distributed system, use a better ID generator.
	msgID := int64(len(r.Messages) + 1)
	
	msg := Message{
		ID:          msgID,
		SenderID:    senderID,
		Content:     content,
		Timestamp:   time.Now(),
		RecipientID: recipientID,
	}
	
	r.Messages = append(r.Messages, msg)
	
	// Notify listeners
	close(r.broadcast)
	r.broadcast = make(chan struct{})
	
	return msgID, nil
}

// GetMessages returns messages after the given lastID.
// If no new messages, it waits up to timeout.
func (r *Room) GetMessages(ctx context.Context, lastID int64, timeout time.Duration) ([]Message, error) {
	// Fast path: check if there are already new messages
	r.mu.RLock()
	if int64(len(r.Messages)) > lastID {
		msgs := make([]Message, len(r.Messages)-int(lastID))
		copy(msgs, r.Messages[lastID:])
		r.mu.RUnlock()
		return msgs, nil
	}
	
	// Wait for new messages
	waitCh := r.broadcast
	r.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, nil // Timeout, no new messages
	case <-waitCh:
		// Woke up, check messages again
		return r.GetMessages(ctx, lastID, 0) // Recurse with 0 timeout to just fetch
	}
}

// GetLatestMessage returns the last message in the room, or nil if empty
func (r *Room) GetLatestMessage() *Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.Messages) == 0 {
		return nil
	}
	return &r.Messages[len(r.Messages)-1]
}

// UpdateMemberLastMsgID updates the cursor for a member
func (r *Room) UpdateMemberLastMsgID(memberID string, lastMsgID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if member, ok := r.Members[memberID]; ok {
		member.LastMsgID = lastMsgID
	}
}

// GetMemberRole returns the role of a member, or empty string if not found
func (r *Room) GetMemberRole(memberID string) Role {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if member, ok := r.Members[memberID]; ok {
		return member.Role
	}
	return ""
}
