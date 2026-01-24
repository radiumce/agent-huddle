package huddle

import (
	"errors"
	"time"
)

var (
	ErrRoomClosed     = errors.New("room is closed")
	ErrContextChanged = errors.New("context changed, please update")
	ErrMemberNotFound = errors.New("member not found")
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

// postSystemMessage posts a message from [System] without locking (caller must hold lock).
func (r *Room) postSystemMessageLocked(content string) {
	msgID := int64(len(r.Messages) + 1)
	msg := Message{
		ID:        msgID,
		Sender:    "[System]",
		Content:   content,
		Timestamp: time.Now(),
	}
	r.Messages = append(r.Messages, msg)
	r.LastActive = time.Now()

	// Notify waiters
	close(r.broadcastChan)
	r.broadcastChan = make(chan struct{})
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

// LeaveRoom marks a member as having left the room and posts a notification.
func (r *Room) LeaveRoom(memberName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	member, ok := r.Members[memberName]
	if !ok {
		return ErrMemberNotFound
	}

	member.Left = true
	member.WaitingSince = nil
	member.Active = false

	r.postSystemMessageLocked("[" + memberName + "] leaved room.")
	return nil
}

// WaitForMessage blocks until a new message arrives or timeout.
func (r *Room) WaitForMessage(memberID string, lastMsgID int64, timeout time.Duration) ([]Message, error) {
	// Start heartbeat goroutine on first wait
	r.heartbeatStart.Do(func() {
		go r.heartbeatLoop()
	})

	// Use write lock since we need to modify WaitingSince
	r.mu.Lock()
	// Check if we already have new messages
	if len(r.Messages) > 0 && r.Messages[len(r.Messages)-1].ID > lastMsgID {
		msgs := r.getMessagesSince(lastMsgID)
		r.mu.Unlock()
		return msgs, nil
	}

	// Mark member as waiting
	if member, ok := r.Members[memberID]; ok {
		now := time.Now()
		member.WaitingSince = &now
	}

	// Capture the current broadcast channel to wait on
	waitChan := r.broadcastChan
	r.mu.Unlock()

	// Defer clearing wait state
	defer func() {
		r.mu.Lock()
		if member, ok := r.Members[memberID]; ok {
			member.WaitingSince = nil
		}
		r.mu.Unlock()
	}()

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

// heartbeatLoop runs in background and sends heartbeat when all active members are waiting.
func (r *Room) heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkAndSendHeartbeat()
		case <-r.heartbeatStop:
			return
		case <-r.closeChan:
			return
		}
	}
}

// checkAndSendHeartbeat checks if all active (non-left) members are waiting for HeartbeatInterval.
// Only triggers when there are >= 2 active members all waiting.
func (r *Room) checkAndSendHeartbeat() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	allWaiting := true
	activeMemberCount := 0

	for _, member := range r.Members {
		if member.Left {
			continue
		}
		activeMemberCount++

		if member.WaitingSince == nil {
			allWaiting = false
			break
		}
		if now.Sub(*member.WaitingSince) < r.HeartbeatInterval {
			allWaiting = false
			break
		}
	}

	// Only send heartbeat if >= 2 active members and all are waiting
	if activeMemberCount >= 2 && allWaiting {
		r.postSystemMessageLocked(r.HeartbeatMessage)
		// Reset WaitingSince for all members to avoid repeated heartbeats
		for _, member := range r.Members {
			if !member.Left && member.WaitingSince != nil {
				nowReset := time.Now()
				member.WaitingSince = &nowReset
			}
		}
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

	// Also stop heartbeat
	select {
	case <-r.heartbeatStop:
	default:
		close(r.heartbeatStop)
	}
}
