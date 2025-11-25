package huddle

import (
	"errors"
	"time"
)

var (
	ErrRoomClosed     = errors.New("room is closed")
	ErrContextChanged = errors.New("context changed, please update")
)

// PostMessage adds a message to the room.
// If force=false and new messages exist since lastSeenID, returns ErrContextChanged.
// If force=true, posts regardless and returns pre-existing messages.
// Returns: (postedMsgID, preExistingMsgs, error)
func (r *Room) PostMessage(sender string, content string, recipient string, lastSeenID int64, force bool) (int64, []Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-r.closeChan:
		return 0, nil, ErrRoomClosed
	default:
	}

	// Collect messages that arrived since lastSeenID
	var preExistingMsgs []Message
	for _, m := range r.Messages {
		if m.ID > lastSeenID {
			preExistingMsgs = append(preExistingMsgs, m)
		}
	}

	// Optimistic Concurrency Control (only when not forcing)
	if !force && len(preExistingMsgs) > 0 {
		return 0, preExistingMsgs, ErrContextChanged
	}

	msgID := int64(len(r.Messages) + 1)
	msg := Message{
		ID:        msgID,
		Sender:    sender,
		Recipient: recipient,
		Content:   content,
		Timestamp: time.Now(),
	}

	r.Messages = append(r.Messages, msg)
	r.LastActive = time.Now()

	if member, ok := r.Members[sender]; ok {
		member.LastMsgID = msgID
		member.LastActiveAt = time.Now()
	}

	// Notify waiters by closing the old channel and creating a new one
	close(r.broadcastChan)
	r.broadcastChan = make(chan struct{})
	return msgID, preExistingMsgs, nil
}

// EnsureMember ensures the member exists in the room. Idempotent.
func (r *Room) EnsureMember(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Members[name]; !ok {
		r.Members[name] = &Member{
			Name:         name,
			IsHost:       false,
			Active:       true,
			LastActiveAt: time.Now(),
		}
	}
}

// WaitForMessage blocks until a new message arrives or timeout.
func (r *Room) WaitForMessage(memberID string, lastMsgID int64, timeout time.Duration) ([]Message, error) {
	r.mu.RLock()
	// Check if we already have new messages
	if len(r.Messages) > 0 && r.Messages[len(r.Messages)-1].ID > lastMsgID {
		msgs := r.getMessagesSince(lastMsgID)
		r.mu.RUnlock()
		return msgs, nil
	}

	// Capture the current broadcast channel to wait on
	waitChan := r.broadcastChan
	r.mu.RUnlock()

	select {
	case <-waitChan:
		// Woken up, check messages
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.getMessagesSince(lastMsgID), nil
	case <-time.After(timeout):
		return nil, nil // Timeout, no error, just empty result
	case <-r.closeChan:
		return nil, ErrRoomClosed
	}
}

func (r *Room) getMessagesSince(lastID int64) []Message {
	var result []Message
	for _, m := range r.Messages {
		if m.ID > lastID {
			result = append(result, m)
		}
	}
	return result
}

func (r *Room) Join(member *Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Members[member.Name]; !ok {
		r.Members[member.Name] = member
	}
}

func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.closeChan:
		return
	default:
		close(r.closeChan)
	}
}
