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
	
	// Concurrency control
	mu            sync.RWMutex
	broadcastChan chan struct{} // Closed when new message arrives
	closeChan     chan struct{}
}

func NewRoom(id, number, name string, host *Member) *Room {
	r := &Room{
		ID:            id,
		Number:        number,
		Name:          name,
		Host:          host,
		Members:       make(map[string]*Member),
		Messages:      make([]Message, 0),
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
		broadcastChan: make(chan struct{}),
		closeChan:     make(chan struct{}),
	}
	r.Members[host.Name] = host
	return r
}
