package api

import (
	"errors"
	"net/http"
	"time"
)

type drainRequest struct {
	Basin string `json:"basin"`
}

type levelRequest struct {
	Basin  string  `json:"basin"`
	Meters float64 `json:"meters"`
}

type reverseGateRequest struct {
	Gate string `json:"gate"`
}

type stopPumpRequest struct {
	Pump   string `json:"pump"`
	Reason string `json:"reason"`
}

func (server *Server) equipment(writer http.ResponseWriter, request *http.Request) {
	status := server.runtime.Status(time.Now())
	writeJSON(writer, http.StatusOK, equipmentResponse(status))
}

func (server *Server) startDrain(writer http.ResponseWriter, request *http.Request) {
	var input drainRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Basin == "" {
		writeError(writer, http.StatusBadRequest, errors.New("basin is required"))
		return
	}
	result, err := server.runtime.Drain(input.Basin, time.Now())
	if err != nil {
		writeError(writer, http.StatusConflict, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, result)
}

func (server *Server) updateLevel(writer http.ResponseWriter, request *http.Request) {
	var input levelRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Basin == "" {
		writeError(writer, http.StatusBadRequest, errors.New("basin is required"))
		return
	}
	item, err := server.runtime.UpdateBasinLevel(input.Basin, input.Meters, time.Now())
	if err != nil {
		writeError(writer, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(writer, http.StatusOK, item)
}

func (server *Server) reverseGate(writer http.ResponseWriter, request *http.Request) {
	var input reverseGateRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Gate == "" {
		writeError(writer, http.StatusBadRequest, errors.New("gate is required"))
		return
	}
	gate, err := server.runtime.ReverseGate(request.Context(), input.Gate, time.Now())
	if err != nil {
		writeError(writer, http.StatusConflict, err)
		return
	}
	writeJSON(writer, http.StatusOK, gate)
}

func (server *Server) stopPump(writer http.ResponseWriter, request *http.Request) {
	var input stopPumpRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.Pump == "" || input.Reason == "" {
		writeError(writer, http.StatusBadRequest, errors.New("pump and reason are required"))
		return
	}
	pump, err := server.runtime.StopPump(input.Pump, input.Reason, time.Now())
	if err != nil {
		writeError(writer, http.StatusConflict, err)
		return
	}
	writeJSON(writer, http.StatusOK, pump)
}
