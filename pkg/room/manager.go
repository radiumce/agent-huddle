package room

import (
	"fmt"
	"sync"
	"time"
)

// Manager manages multiple rooms
type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewManager creates a new room manager
func NewManager() *Manager {
	m := &Manager{
		rooms: make(map[string]*Room),
	}
	go m.cleanupLoop()
	return m
}

// CreateRoom creates a new room with a given ID and name
func (m *Manager) CreateRoom(id, name string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[id]; exists {
		return nil, fmt.Errorf("room with id %s already exists", id)
	}

	room := NewRoom(id, name)
	m.rooms[id] = room
	return room, nil
}

// GetRoom retrieves a room by ID
func (m *Manager) GetRoom(id string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, exists := m.rooms[id]
	if !exists {
		return nil, fmt.Errorf("room %s not found", id)
	}
	return room, nil
}

// ListRooms returns a list of all active rooms
func (m *Manager) ListRooms() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

// CloseRoom removes a room
func (m *Manager) CloseRoom(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, id)
}

// cleanupLoop periodically removes inactive rooms
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		m.mu.Lock()
		for id, r := range m.rooms {
			r.mu.RLock()
			lastActivity := r.LastActivity
			r.mu.RUnlock()

			if time.Since(lastActivity) > 30*time.Minute {
				delete(m.rooms, id)
				fmt.Printf("Room %s cleaned up due to inactivity\n", id)
			}
		}
		m.mu.Unlock()
	}
}
