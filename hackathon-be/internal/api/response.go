package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError maps service-layer errors to HTTP status codes.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	var status int
	switch {
	case errors.Is(err, service.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	default:
		s.log.Error("request failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decode reads a JSON body into dst, writing a 400 on failure.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.writeError(w, fmt.Errorf("%w: invalid JSON body: %v", service.ErrValidation, err))
		return false
	}
	return true
}

// pathID extracts and parses a UUID path parameter, writing a 400 on failure.
func (s *Server) pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		s.writeError(w, fmt.Errorf("%w: invalid %s", service.ErrValidation, name))
		return uuid.Nil, false
	}
	return id, true
}

// nonNil returns an empty slice instead of nil so JSON encodes "[]" not "null".
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// errValidation builds a validation error with the given message.
func errValidation(msg string) error {
	return fmt.Errorf("%w: %s", service.ErrValidation, msg)
}
