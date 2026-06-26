package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/service"
)

type rangeReq struct {
	MaxRangeM *int `json:"maxRangeM"`
}

type centerReq struct {
	Lat *float64 `json:"lat"`
	Lng *float64 `json:"lng"`
}

type messageReq struct {
	UserIDs []string `json:"userIds"`
	Body    string   `json:"body"`
}

type notifyReq struct {
	Body string `json:"body"`
}

type checkinReq struct {
	OK           *bool    `json:"ok"`
	Lat          *float64 `json:"lat"`
	Lng          *float64 `json:"lng"`
	BatteryLevel *int     `json:"batteryLevel"`
}

func (s *Server) getNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	view, err := s.svc.GetNightView(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) startNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.svc.StartNight(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) endNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.svc.EndNight(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) deleteNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.svc.DeleteNight(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setRange(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req rangeReq
	if !s.decode(w, r, &req) {
		return
	}
	if req.MaxRangeM == nil {
		s.writeError(w, errValidation("maxRangeM is required"))
		return
	}
	n, err := s.svc.SetRange(r.Context(), id, *req.MaxRangeM)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) setCenter(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req centerReq
	if !s.decode(w, r, &req) {
		return
	}
	if req.Lat == nil || req.Lng == nil {
		s.writeError(w, errValidation("lat and lng are required"))
		return
	}
	n, err := s.svc.SetCenter(r.Context(), id, *req.Lat, *req.Lng)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) checkNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	res, err := s.svc.Check(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) checkinNight(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := s.pathID(w, r, "userId")
	if !ok {
		return
	}
	var req checkinReq
	if !s.decode(w, r, &req) {
		return
	}
	if req.OK == nil {
		s.writeError(w, errValidation("ok is required"))
		return
	}
	st, err := s.svc.Checkin(r.Context(), id, userID, service.CheckinInput{
		OK:      *req.OK,
		Lat:     req.Lat,
		Lng:     req.Lng,
		Battery: req.BatteryLevel,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) listNightLocations(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	locs, err := s.svc.ListNightLocations(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(locs))
}

func (s *Server) getNightLocation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := s.pathID(w, r, "userId")
	if !ok {
		return
	}
	loc, err := s.svc.GetLocationOf(r.Context(), id, userID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, loc)
}

func (s *Server) putNightLocation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := s.pathID(w, r, "userId")
	if !ok {
		return
	}
	var req locationReq
	if !s.decode(w, r, &req) {
		return
	}
	if req.Lat == nil || req.Lng == nil {
		s.writeError(w, errValidation("lat and lng are required"))
		return
	}
	loc, err := s.svc.ReportNightLocation(r.Context(), id, userID, *req.Lat, *req.Lng, req.BatteryLevel)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, loc)
}

func (s *Server) listNightStatuses(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	statuses, err := s.svc.ListParticipantStatuses(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(statuses))
}

func (s *Server) getNightBattery(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := s.pathID(w, r, "userId")
	if !ok {
		return
	}
	level, err := s.svc.GetBatteryLevelOf(r.Context(), id, userID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batteryLevel": level})
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req messageReq
	if !s.decode(w, r, &req) {
		return
	}
	userIDs := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, raw := range req.UserIDs {
		uid, err := uuid.Parse(raw)
		if err != nil {
			s.writeError(w, errValidation("invalid userId in userIds: "+raw))
			return
		}
		userIDs = append(userIDs, uid)
	}
	msgs, err := s.svc.Message(r.Context(), id, userIDs, req.Body)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, nonNil(msgs))
}

func (s *Server) notifyAll(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req notifyReq
	if !s.decode(w, r, &req) {
		return
	}
	msgs, err := s.svc.NotifyAll(r.Context(), id, req.Body)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, nonNil(msgs))
}

func (s *Server) listNightMessages(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	msgs, err := s.svc.ListMessages(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(msgs))
}
