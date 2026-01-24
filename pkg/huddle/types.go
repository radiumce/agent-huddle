package huddle

import (
	"sync"
	"time"
)

// Message represents a communication in the meeting room.
type Message struct {
	ID        int64     `json:"id"`
	Sender    string    `json:"sender"`
	Recipient string    `json:"recipient,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Member represents an agent in the meeting room.
type Member struct {
	Name         string
	LastMsgID    int64 // Cursor for the last message received
	IsHost       bool
	Active       bool
	Left         bool       // Whether the member has explicitly left
	WaitingSince *time.Time // When the member started waiting (nil if not waiting)
	LastActiveAt time.Time
}

// Room represents a meeting room.
type Room struct {
	ID         string
	Number     string
	Name       string
	Host       *Member
	Members    map[string]*Member
	Messages   []Message
	CreatedAt  time.Time
	LastActive time.Time

	// Heartbeat settings
	HeartbeatInterval time.Duration // Default 30s
	HeartbeatMessage  string        // Custom heartbeat message

	// Concurrency control
	mu             sync.RWMutex
	broadcastChan  chan struct{} // Closed when new message arrives
	closeChan      chan struct{}
	heartbeatStop  chan struct{} // Stop heartbeat goroutine
	heartbeatStart sync.Once     // Ensure heartbeat starts only once
}

const DefaultHeartbeatInterval = 30 * time.Second
const DefaultHeartbeatMessage = "Meeting heartbeat: All participants are waiting. Please confirm your next action - continue waiting for messages, or proceed with your next step then continue meeting when ready."

func NewRoom(id, number, name string, host *Member) *Room {
	r := &Room{
		ID:                id,
		Number:            number,
		Name:              name,
		Host:              host,
		Members:           make(map[string]*Member),
		Messages:          make([]Message, 0),
		CreatedAt:         time.Now(),
		LastActive:        time.Now(),
		HeartbeatInterval: DefaultHeartbeatInterval,
		HeartbeatMessage:  DefaultHeartbeatMessage,
		broadcastChan:     make(chan struct{}),
		closeChan:         make(chan struct{}),
		heartbeatStop:     make(chan struct{}),
	}
	r.Members[host.Name] = host
	return r
}
