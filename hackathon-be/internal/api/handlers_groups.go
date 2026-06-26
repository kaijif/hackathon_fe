package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/service"
)

type createGroupReq struct {
	Name          string `json:"name"`
	CreatorUserID string `json:"creatorUserId"`
}

type addMemberReq struct {
	UserID string `json:"userId"`
}

type setAdminReq struct {
	UserID  string `json:"userId"`
	IsAdmin *bool  `json:"isAdmin"`
}

type coordsReq struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type createNightReq struct {
	AgentID             *string    `json:"agentId"`
	Center              *coordsReq `json:"center"`
	TimeLimitMin        *int       `json:"timeLimitMin"`
	CheckInLimitMin     *int       `json:"checkInLimitMin"`
	CheckInEveryMin     *int       `json:"checkInEveryMin"`
	MaxRangeM           *int       `json:"maxRangeM"`
	LowBatteryThreshold *int       `json:"lowBatteryThreshold"`
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupReq
	if !s.decode(w, r, &req) {
		return
	}
	creatorID, err := uuid.Parse(req.CreatorUserID)
	if err != nil {
		s.writeError(w, errValidation("valid creatorUserId is required"))
		return
	}
	g, err := s.svc.FormGroup(r.Context(), req.Name, creatorID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.svc.ListGroups(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(groups))
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	g, err := s.svc.GetGroup(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.svc.DeleteGroup(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	members, err := s.svc.ListMembers(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(members))
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req addMemberReq
	if !s.decode(w, r, &req) {
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		s.writeError(w, errValidation("valid userId is required"))
		return
	}
	if err := s.svc.JoinGroup(r.Context(), groupID, userID); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) joinGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req addMemberReq
	if !s.decode(w, r, &req) {
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		s.writeError(w, errValidation("valid userId is required"))
		return
	}
	if err := s.svc.JoinGroup(r.Context(), groupID, userID); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := s.pathID(w, r, "userId")
	if !ok {
		return
	}
	if err := s.svc.LeaveGroup(r.Context(), groupID, userID); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getAdmins(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	admins, err := s.svc.GetAdmins(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(admins))
}

func (s *Server) setAdmin(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req setAdminReq
	if !s.decode(w, r, &req) {
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		s.writeError(w, errValidation("valid userId is required"))
		return
	}
	if req.IsAdmin == nil {
		s.writeError(w, errValidation("isAdmin is required"))
		return
	}
	if err := s.svc.SetAdmin(r.Context(), groupID, userID, *req.IsAdmin); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getCurrentNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.svc.CurrentNight(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) listGroupNights(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	nights, err := s.svc.ListNightsByGroup(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(nights))
}

func (s *Server) createNight(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req createNightReq
	if !s.decode(w, r, &req) {
		return
	}
	in := service.CreateNightInput{
		TimeLimitMin:        req.TimeLimitMin,
		CheckInLimitMin:     req.CheckInLimitMin,
		CheckInEveryMin:     req.CheckInEveryMin,
		MaxRangeM:           req.MaxRangeM,
		LowBatteryThreshold: req.LowBatteryThreshold,
	}
	if req.AgentID != nil && *req.AgentID != "" {
		agentID, err := uuid.Parse(*req.AgentID)
		if err != nil {
			s.writeError(w, errValidation("invalid agentId"))
			return
		}
		in.AgentID = &agentID
	}
	if req.Center != nil {
		in.Center = &models.Coords{Lat: req.Center.Lat, Lng: req.Center.Lng}
	}
	n, err := s.svc.CreateNight(r.Context(), groupID, in)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	a, err := s.svc.CreateAgent(r.Context(), req.Name, req.Description)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.svc.ListAgents(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(agents))
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	a, err := s.svc.GetAgent(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
