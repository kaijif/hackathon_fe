// Package api implements the HTTP transport layer.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/t-kaijifu/hackathon-be/internal/service"
)

// Server wires the service layer to HTTP handlers.
type Server struct {
	svc *service.Service
	log *slog.Logger
}

// NewServer constructs a Server.
func NewServer(svc *service.Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{svc: svc, log: log}
}

// Handler returns the fully-routed HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.health)

	// Users
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("GET /users", s.listUsers)
	mux.HandleFunc("GET /users/{id}", s.getUser)
	mux.HandleFunc("GET /users/{id}/location", s.getUserLocation)
	mux.HandleFunc("PUT /users/{id}/location", s.setUserLocation)
	mux.HandleFunc("GET /users/{id}/battery", s.getUserBattery)
	mux.HandleFunc("PUT /users/{id}/battery", s.setUserBattery)
	mux.HandleFunc("GET /users/{id}/groups", s.listUserGroups)
	mux.HandleFunc("POST /users/{id}/devices", s.registerDevice)
	mux.HandleFunc("GET /users/{id}/devices", s.listDevices)
	mux.HandleFunc("DELETE /users/{id}/devices/{token}", s.unregisterDevice)

	// Groups
	mux.HandleFunc("POST /groups", s.createGroup)
	mux.HandleFunc("GET /groups", s.listGroups)
	mux.HandleFunc("GET /groups/{id}", s.getGroup)
	mux.HandleFunc("DELETE /groups/{id}", s.deleteGroup)
	mux.HandleFunc("GET /groups/{id}/members", s.listMembers)
	mux.HandleFunc("POST /groups/{id}/members", s.addMember)
	mux.HandleFunc("POST /groups/{id}/join", s.joinGroup)
	mux.HandleFunc("DELETE /groups/{id}/members/{userId}", s.removeMember)
	mux.HandleFunc("GET /groups/{id}/admins", s.getAdmins)
	mux.HandleFunc("PUT /groups/{id}/admins", s.setAdmin)
	mux.HandleFunc("GET /groups/{id}/night", s.getCurrentNight)
	mux.HandleFunc("GET /groups/{id}/nights", s.listGroupNights)
	mux.HandleFunc("POST /groups/{id}/nights", s.createNight)

	// Agents
	mux.HandleFunc("POST /agents", s.createAgent)
	mux.HandleFunc("GET /agents", s.listAgents)
	mux.HandleFunc("GET /agents/{id}", s.getAgent)

	// Nights
	mux.HandleFunc("GET /nights/{id}", s.getNight)
	mux.HandleFunc("POST /nights/{id}/start", s.startNight)
	mux.HandleFunc("POST /nights/{id}/end", s.endNight)
	mux.HandleFunc("DELETE /nights/{id}", s.deleteNight)
	mux.HandleFunc("PUT /nights/{id}/range", s.setRange)
	mux.HandleFunc("PUT /nights/{id}/center", s.setCenter)
	mux.HandleFunc("POST /nights/{id}/check", s.checkNight)
	mux.HandleFunc("POST /nights/{id}/checkin/{userId}", s.checkinNight)
	mux.HandleFunc("GET /nights/{id}/locations", s.listNightLocations)
	mux.HandleFunc("GET /nights/{id}/locations/{userId}", s.getNightLocation)
	mux.HandleFunc("PUT /nights/{id}/locations/{userId}", s.putNightLocation)
	mux.HandleFunc("GET /nights/{id}/statuses", s.listNightStatuses)
	mux.HandleFunc("GET /nights/{id}/battery/{userId}", s.getNightBattery)
	mux.HandleFunc("POST /nights/{id}/messages", s.postMessage)
	mux.HandleFunc("GET /nights/{id}/messages", s.listNightMessages)
	mux.HandleFunc("POST /nights/{id}/notify", s.notifyAll)

	return s.recoverer(s.logger(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// logger logs each request with method, path, status, and duration.
func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// recoverer turns panics into 500 responses instead of crashing the server.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered", "error", rec, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
