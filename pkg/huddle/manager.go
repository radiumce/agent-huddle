package huddle

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomAlreadyExists = errors.New("room already exists")
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

func (m *Manager) CreateRoom(id string, name string, hostName string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If ID provided, check for conflict
	if id != "" {
		if _, ok := m.rooms[id]; ok {
			return nil, ErrRoomAlreadyExists
		}
	} else {
		// Generate simple ID
		id = fmt.Sprintf("room-%d", time.Now().UnixNano())
	}

	// Generate Number
	number := fmt.Sprintf("%06d", rand.Intn(1000000))

	host := &Member{
		Name:         hostName,
		IsHost:       true,
		Active:       true,
		LastActiveAt: time.Now(),
	}

	room := NewRoom(id, number, name, host)
	m.rooms[id] = room
	
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
