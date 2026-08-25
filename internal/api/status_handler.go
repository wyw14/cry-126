package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/wyw14/cry-126/internal/telemetry"
)

type telemetryRequest struct {
	SourceID   string    `json:"source_id"`
	Pressure   float64   `json:"pressure"`
	ObservedAt time.Time `json:"observed_at"`
}

func (server *Server) health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"status":         "ok",
		"service":        "TideShield",
		"uptime_seconds": int(time.Since(server.runtime.StartedAt()).Seconds()),
	})
}

func (server *Server) status(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, server.runtime.Status(time.Now()))
}

func (server *Server) receiveTelemetry(writer http.ResponseWriter, request *http.Request) {
	var input telemetryRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.SourceID == "" {
		writeError(writer, http.StatusBadRequest, errors.New("source_id is required"))
		return
	}
	if input.ObservedAt.IsZero() {
		input.ObservedAt = time.Now()
	}
	now := time.Now().UTC()
	items, err := server.runtime.ReceiveSample(telemetry.Sample{SourceID: input.SourceID, Pressure: input.Pressure, ObservedAt: input.ObservedAt.UTC(), ReceivedAt: now})
	if err != nil {
		writeError(writer, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, items)
}
