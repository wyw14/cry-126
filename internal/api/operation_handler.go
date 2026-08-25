package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/wyw14/cry-126/internal/forecast"
)

type startOperationRequest struct {
	Reason string `json:"reason"`
}

type forecastRequest struct {
	Points []struct {
		At     time.Time `json:"at"`
		Meters float64   `json:"meters"`
	} `json:"points"`
}

func (server *Server) operations(writer http.ResponseWriter, request *http.Request) {
	status := server.runtime.Status(time.Now())
	writeJSON(writer, http.StatusOK, map[string]any{
		"current":    status.Operation,
		"started_at": status.StartedAt,
		"forecast":   status.Forecast,
	})
}

func (server *Server) startOperation(writer http.ResponseWriter, request *http.Request) {
	var input startOperationRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Reason == "" {
		writeError(writer, http.StatusBadRequest, errors.New("reason is required"))
		return
	}
	operation, err := server.runtime.StartOperation(request.Context(), input.Reason, time.Now())
	if err != nil {
		writeError(writer, http.StatusConflict, err)
		return
	}
	writeJSON(writer, http.StatusCreated, operation)
}

func (server *Server) forecast(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, server.runtime.Status(time.Now()).Forecast)
}

func (server *Server) updateForecast(writer http.ResponseWriter, request *http.Request) {
	var input forecastRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	points := make([]forecast.Point, 0, len(input.Points))
	for _, point := range input.Points {
		points = append(points, forecast.Point{At: point.At, Meters: point.Meters})
	}
	window, err := server.runtime.UpdateForecast(points, time.Now())
	if err != nil {
		writeError(writer, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(writer, http.StatusOK, window)
}
