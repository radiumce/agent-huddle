package api

import (
	"encoding/json"
	"net/http"

	"github.com/chene/agent-huddle/pkg/huddle"
)

type Server struct {
	service *huddle.Service
}

func NewServer(service *huddle.Service) *Server {
	return &Server{service: service}
}

// Request Models
type CreateRoomRequest struct {
	RoomID      string `json:"room_id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	InitMessage string `json:"init_message"`
	TimeoutSec  *int   `json:"timeout_sec,omitempty"`
}

type PostMessageRequest struct {
	RoomID     string `json:"room_id"`
	Sender     string `json:"sender"`
	Content    string `json:"content"`
	Recipient  string `json:"recipient"`
	LastID     int64  `json:"last_id"`
	TimeoutSec *int   `json:"timeout_sec,omitempty"`
	Force      bool   `json:"force"`
}

type WaitForMessageRequest struct {
	RoomID     string `json:"room_id"`
	MemberName string `json:"member_name"`
	LastID     int64  `json:"last_id"`
	TimeoutSec *int   `json:"timeout_sec,omitempty"`
}

type RoomContextRequest struct {
	RoomID     string `json:"room_id"`
	MemberName string `json:"member_name"`
	LastID     int64  `json:"last_id"`
}

type CloseRoomRequest struct {
	RoomID string `json:"room_id"`
}

type LeaveRoomRequest struct {
	RoomID     string `json:"room_id"`
	MemberName string `json:"member_name"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/rooms/create_and_wait", s.handleCreateAndWait)
	mux.HandleFunc("/api/rooms/list", s.handleListRooms)
	mux.HandleFunc("/api/rooms/post_message_and_wait", s.handlePostMessageAndWait)
	mux.HandleFunc("/api/rooms/force_post_message_and_wait", s.handleForcePostMessageAndWait)
	mux.HandleFunc("/api/rooms/wait_for_message", s.handleWaitForMessage)
	mux.HandleFunc("/api/rooms/context", s.handleRoomContext)
	mux.HandleFunc("/api/rooms/close", s.handleCloseRoom)
	mux.HandleFunc("/api/rooms/leave", s.handleLeaveRoom)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleCreateAndWait(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	timeout := 300
	if req.TimeoutSec != nil {
		timeout = *req.TimeoutSec
	}
	resp, err := s.service.CreateRoomAndWait(r.Context(), req.RoomID, req.Name, req.Host, req.InitMessage, timeout)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	resp, err := s.service.ListRooms(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePostMessageAndWait(w http.ResponseWriter, r *http.Request) {
	s.doPostMessageAndWait(w, r, false)
}

func (s *Server) handleForcePostMessageAndWait(w http.ResponseWriter, r *http.Request) {
	s.doPostMessageAndWait(w, r, true)
}

func (s *Server) doPostMessageAndWait(w http.ResponseWriter, r *http.Request, forceOverride bool) {
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	timeout := 300
	if req.TimeoutSec != nil {
		timeout = *req.TimeoutSec
	}
	force := req.Force || forceOverride
	resp, err := s.service.PostMessageAndWait(r.Context(), req.RoomID, req.Sender, req.Content, req.Recipient, req.LastID, timeout, force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWaitForMessage(w http.ResponseWriter, r *http.Request) {
	var req WaitForMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	timeout := 300
	if req.TimeoutSec != nil {
		timeout = *req.TimeoutSec
	}
	resp, err := s.service.WaitForMessage(r.Context(), req.RoomID, req.MemberName, req.LastID, timeout)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRoomContext(w http.ResponseWriter, r *http.Request) {
	var req RoomContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.service.GetRoomContext(r.Context(), req.RoomID, req.MemberName, req.LastID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCloseRoom(w http.ResponseWriter, r *http.Request) {
	var req CloseRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.service.CloseRoom(r.Context(), req.RoomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLeaveRoom(w http.ResponseWriter, r *http.Request) {
	var req LeaveRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.service.LeaveRoom(r.Context(), req.RoomID, req.MemberName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
