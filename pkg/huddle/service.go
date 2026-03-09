package huddle

import (
	"context"
	"fmt"
	"time"
)

// Service encapsulates the high-level workflows spanning the Manager and Room operations.
type Service struct {
	Manager *Manager
}

func NewService(manager *Manager) *Service {
	if manager == nil {
		manager = NewManager()
	}
	return &Service{
		Manager: manager,
	}
}

// ----- Response Types -----

type ContextResponse struct {
	Messages []Message `json:"messages"`
}

type CreateRoomResponse struct {
	Result    string    `json:"result"`
	RoomID    string    `json:"room_id"`
	Messages  []Message `json:"messages"`
	LastMsgID int64     `json:"last_msg_id"`
}

type PostMessageResponse struct {
	Result          string    `json:"result"`
	Messages        []Message `json:"messages"`
	PreExistingMsgs []Message `json:"pre_existing_msgs,omitempty"`
	HadPreExisting  bool      `json:"had_pre_existing"`
	LastMsgID       int64     `json:"last_msg_id,omitempty"`
}

type ListRoomsResponse struct {
	Rooms []*Room `json:"rooms"`
}

type CloseRoomResponse struct {
	Result string `json:"result"`
}

type LeaveRoomResponse struct {
	Result string `json:"result"`
}

// ----- Service Methods -----

func (s *Service) GetRoomContext(ctx context.Context, roomID, memberName string, lastMsgID int64) (*ContextResponse, error) {
	room, err := s.Manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	room.EnsureMember(memberName)

	msgs, err := room.WaitForMessage(memberName, lastMsgID, 0)
	if err != nil {
		return nil, err
	}

	if msgs == nil {
		msgs = []Message{}
	}
	return &ContextResponse{Messages: msgs}, nil
}

func (s *Service) CreateRoomAndWait(ctx context.Context, roomID, name, host, initMessage string, timeoutSec int) (*CreateRoomResponse, error) {
	room, err := s.Manager.CreateRoom(roomID, name, host)
	if err != nil {
		if err == ErrRoomAlreadyExists {
			// Idempotency: Try to get the existing room
			existingRoom, getErr := s.Manager.GetRoom(roomID)
			if getErr != nil {
				return &CreateRoomResponse{
					Result:    fmt.Sprintf("Room ID conflict: %s already exists and could not be retrieved.", roomID),
					Messages:  []Message{},
					LastMsgID: 0,
				}, nil
			}
			room = existingRoom
		} else {
			return nil, err
		}
	}

	lastMsgID := int64(0)
	if initMessage != "" {
		room.EnsureMember(host)

		// Get latest state to check for duplicates and get correct LastMsgID
		msgs, _ := room.WaitForMessage(host, 0, 0)

		isDuplicate := false
		currentLastID := int64(0)
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			currentLastID = lastMsg.ID
			if lastMsg.Sender == host && lastMsg.Content == initMessage {
				isDuplicate = true
				lastMsgID = lastMsg.ID
			}
		}

		if !isDuplicate {
			id, _, err := room.PostMessage(host, initMessage, "", currentLastID, false)
			if err != nil {
				if err == ErrContextChanged {
					msgs, _ := room.WaitForMessage(host, 0, 0)
					if len(msgs) > 0 {
						currentLastID = msgs[len(msgs)-1].ID
					}
					id, _, err = room.PostMessage(host, initMessage, "", currentLastID, false)
					if err != nil {
						return nil, fmt.Errorf("failed to post init message after retry: %w", err)
					}
					lastMsgID = id
				} else {
					return nil, err
				}
			} else {
				lastMsgID = id
			}
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second
	msgs, err := room.WaitForMessage(host, lastMsgID, timeout)
	if err != nil {
		if err == ErrRoomClosed {
			return &CreateRoomResponse{
				Result:    "Room closed",
				RoomID:    room.ID,
				Messages:  []Message{},
				LastMsgID: lastMsgID,
			}, nil
		}
		return nil, err
	}

	if msgs == nil {
		msgs = []Message{}
	}

	finalLastMsgID := lastMsgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return &CreateRoomResponse{
		Result:    fmt.Sprintf("Room created (or joined) and waited. ID: %s", room.ID),
		RoomID:    room.ID,
		Messages:  msgs,
		LastMsgID: finalLastMsgID,
	}, nil
}

func (s *Service) PostMessageAndWait(ctx context.Context, roomID, sender, content, recipient string, lastSeenID int64, timeoutSec int, force bool) (*PostMessageResponse, error) {
	room, err := s.Manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	room.EnsureMember(sender)

	msgID, preExistingMsgs, err := room.PostMessage(sender, content, recipient, lastSeenID, force)
	if err != nil {
		if err == ErrContextChanged {
			return &PostMessageResponse{
				Result:          "Post rejected: New messages available. Please review and retry.",
				PreExistingMsgs: preExistingMsgs,
				HadPreExisting:  true,
			}, nil
		}
		if err == ErrRoomClosed {
			return &PostMessageResponse{
				Result:   "Room closed",
				Messages: []Message{},
			}, nil
		}
		return nil, err
	}

	if force && len(preExistingMsgs) > 0 {
		return &PostMessageResponse{
			Result:          "Message posted. Pre-existing messages found since last_seen_id.",
			PreExistingMsgs: preExistingMsgs,
			HadPreExisting:  true,
			LastMsgID:       msgID,
		}, nil
	}

	timeout := time.Duration(timeoutSec) * time.Second
	msgs, err := room.WaitForMessage(sender, msgID, timeout)
	if err != nil {
		if err == ErrRoomClosed {
			return &PostMessageResponse{
				Result:         "Room closed",
				Messages:       []Message{},
				HadPreExisting: false,
				LastMsgID:      msgID,
			}, nil
		}
		return nil, err
	}

	if msgs == nil {
		msgs = []Message{}
	}

	finalLastMsgID := msgID
	if len(msgs) > 0 {
		finalLastMsgID = msgs[len(msgs)-1].ID
	}

	return &PostMessageResponse{
		Result:         "Message posted and waited",
		Messages:       msgs,
		HadPreExisting: false,
		LastMsgID:      finalLastMsgID,
	}, nil
}

func (s *Service) WaitForMessage(ctx context.Context, roomID, memberName string, lastMsgID int64, timeoutSec int) (*ContextResponse, error) {
	timeout := time.Duration(timeoutSec) * time.Second

	room, err := s.Manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	room.EnsureMember(memberName)

	msgs, err := room.WaitForMessage(memberName, lastMsgID, timeout)
	if err != nil {
		if err == ErrRoomClosed {
			return &ContextResponse{
				Messages: []Message{},
			}, nil
		}
		return nil, err
	}

	if msgs == nil {
		msgs = []Message{}
	}

	return &ContextResponse{Messages: msgs}, nil
}

func (s *Service) ListRooms(ctx context.Context) (*ListRoomsResponse, error) {
	rooms := s.Manager.ListRooms()
	if rooms == nil {
		rooms = []*Room{}
	}
	return &ListRoomsResponse{Rooms: rooms}, nil
}

func (s *Service) CloseRoom(ctx context.Context, roomID string) (*CloseRoomResponse, error) {
	room, err := s.Manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}
	room.Close()
	return &CloseRoomResponse{Result: "Room closed"}, nil
}

func (s *Service) LeaveRoom(ctx context.Context, roomID, memberName string) (*LeaveRoomResponse, error) {
	room, err := s.Manager.GetRoom(roomID)
	if err != nil {
		return nil, err
	}

	err = room.LeaveRoom(memberName)
	if err != nil {
		return nil, err
	}

	return &LeaveRoomResponse{
		Result: fmt.Sprintf("Member '%s' has left the room", memberName),
	}, nil
}
