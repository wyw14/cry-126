package api

import (
	"errors"
	"net/http"
	"time"
)

type releaseRequest struct {
	Generation uint64 `json:"generation"`
}

func (server *Server) interlocks(writer http.ResponseWriter, request *http.Request) {
	status := server.runtime.Status(time.Now())
	writeJSON(writer, http.StatusOK, map[string]any{
		"permit":     status.Permit,
		"assessment": status.Assessment,
		"locks":      status.Locks,
		"barrier":    status.Barrier,
	})
}

func (server *Server) emergencyClose(writer http.ResponseWriter, request *http.Request) {
	decision, err := server.runtime.EmergencyClose(request.Context(), time.Now())
	if err != nil {
		writeJSON(writer, http.StatusConflict, map[string]any{"error": err.Error(), "decision": decision})
		return
	}
	writeJSON(writer, http.StatusOK, decision)
}

func (server *Server) releaseClosure(writer http.ResponseWriter, request *http.Request) {
	var input releaseRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Generation == 0 {
		writeError(writer, http.StatusBadRequest, errors.New("generation is required"))
		return
	}
	if !server.runtime.ReleaseClosure(input.Generation) {
		writeError(writer, http.StatusConflict, errors.New("closure lock was not released"))
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"released": true, "generation": input.Generation})
}

func (server *Server) incidents(writer http.ResponseWriter, request *http.Request) {
	status := server.runtime.Status(time.Now())
	writeJSON(writer, http.StatusOK, incidentResponse(status, server.runtime.RecoveryBarriers()))
}
