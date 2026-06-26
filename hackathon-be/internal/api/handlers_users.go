package api

import (
	"net/http"
	"time"

	"github.com/t-kaijifu/hackathon-be/internal/service"
)

type createUserReq struct {
	Name           string   `json:"name"`
	TrustedContact string   `json:"trustedContact"`
	Lat            *float64 `json:"lat"`
	Lng            *float64 `json:"lng"`
	BatteryLevel   *int     `json:"batteryLevel"`
}

type locationReq struct {
	Lat          *float64 `json:"lat"`
	Lng          *float64 `json:"lng"`
	BatteryLevel *int     `json:"batteryLevel"`
}

type batteryReq struct {
	BatteryLevel *int `json:"batteryLevel"`
}

type locationResp struct {
	Lat          *float64   `json:"lat"`
	Lng          *float64   `json:"lng"`
	BatteryLevel *int       `json:"batteryLevel"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if !s.decode(w, r, &req) {
		return
	}
	u, err := s.svc.CreateUser(r.Context(), service.CreateUserInput{
		Name:           req.Name,
		TrustedContact: req.TrustedContact,
		Lat:            req.Lat,
		Lng:            req.Lng,
		Battery:        req.BatteryLevel,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(users))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	u, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) getUserLocation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	u, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, locationResp{
		Lat: u.Lat, Lng: u.Lng, BatteryLevel: u.BatteryLevel, UpdatedAt: u.LocationUpdatedAt,
	})
}

func (s *Server) setUserLocation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
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
	u, err := s.svc.SetLocation(r.Context(), id, *req.Lat, *req.Lng, req.BatteryLevel)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) getUserBattery(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	u, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batteryLevel": u.BatteryLevel})
}

func (s *Server) setUserBattery(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req batteryReq
	if !s.decode(w, r, &req) {
		return
	}
	if req.BatteryLevel == nil {
		s.writeError(w, errValidation("batteryLevel is required"))
		return
	}
	u, err := s.svc.SetBattery(r.Context(), id, *req.BatteryLevel)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) listUserGroups(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	onlyActive := r.URL.Query().Get("activeNight") == "true"
	groups, err := s.svc.ListGroupsForUser(r.Context(), id, onlyActive)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(groups))
}

type registerDeviceReq struct {
	Platform string `json:"platform"`
	Token    string `json:"token"`
}

func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	var req registerDeviceReq
	if !s.decode(w, r, &req) {
		return
	}
	s.log.Info("device registration request received",
		"userId", id, "platform", req.Platform, "token", req.Token)
	dt, err := s.svc.RegisterDevice(r.Context(), id, req.Platform, req.Token)
	if err != nil {
		s.log.Warn("device registration failed",
			"userId", id, "platform", req.Platform, "token", req.Token, "error", err)
		s.writeError(w, err)
		return
	}
	s.log.Info("device registration succeeded",
		"userId", id, "deviceId", dt.ID, "platform", dt.Platform, "token", dt.Token)
	writeJSON(w, http.StatusCreated, dt)
}

func (s *Server) unregisterDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		s.writeError(w, errValidation("token is required"))
		return
	}
	if err := s.svc.UnregisterDevice(r.Context(), id, token); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	devices, err := s.svc.ListDevices(r.Context(), id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(devices))
}
