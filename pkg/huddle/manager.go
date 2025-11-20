package huddle

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	ErrRoomNotFound = errors.New("room not found")
)

type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	m := &Manager{
		rooms: make(map[string]*Room),
	}
	go m.cleanupLoop()
	return m
}

func (m *Manager) CreateRoom(name string, hostName string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate simple ID and Number
	id := fmt.Sprintf("room-%d", time.Now().UnixNano())
	number := fmt.Sprintf("%06d", rand.Intn(1000000))

	host := &Member{
		Name:         hostName,
		IsHost:       true,
		Active:       true,
		LastActiveAt: time.Now(),
	}

	room := NewRoom(id, number, name, host)
	m.rooms[id] = room
	// Also map by number if needed, but for now we just store by ID and search by number if needed
	
	return room, nil
}

func (m *Manager) GetRoom(id string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	room, ok := m.rooms[id]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (m *Manager) GetRoomByNumber(number string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, r := range m.rooms {
		if r.Number == number {
			return r, nil
		}
	}
	return nil, ErrRoomNotFound
}

func (m *Manager) ListRooms() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var list []*Room
	for _, r := range m.rooms {
		list = append(list, r)
	}
	return list
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		m.mu.Lock()
		for id, r := range m.rooms {
			r.mu.RLock()
			idle := time.Since(r.LastActive) > 30*time.Minute
			r.mu.RUnlock()
			
			if idle {
				r.Close()
				delete(m.rooms, id)
			}
		}
		m.mu.Unlock()
	}
}
