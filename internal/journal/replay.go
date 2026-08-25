package journal

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/wyw14/cry-126/internal/model"
)

type ReplayState struct {
	Snapshot model.SystemSnapshot
	Applied  []Event
	Skipped  []Event
}

func Replay(snapshot model.SystemSnapshot, events []Event) (ReplayState, error) {
	state := ReplayState{Snapshot: model.CloneSnapshot(snapshot)}
	ordered := append([]Event(nil), events...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Cursor < ordered[j].Cursor })
	for _, event := range ordered {
		if event.Cursor <= state.Snapshot.Cursor {
			state.Skipped = append(state.Skipped, event)
			continue
		}
		if err := applyEvent(&state.Snapshot, event); err != nil {
			return ReplayState{}, fmt.Errorf("apply cursor %d: %w", event.Cursor, err)
		}
		state.Snapshot.Cursor = event.Cursor
		state.Applied = append(state.Applied, event)
	}
	model.SortSnapshot(&state.Snapshot)
	return state, nil
}

func applyEvent(snapshot *model.SystemSnapshot, event Event) error {
	switch event.Type {
	case "operation.updated":
		var operation model.Operation
		if err := json.Unmarshal(event.Data, &operation); err != nil {
			return err
		}
		snapshot.Operation = operation
	case "gate.updated":
		var gate model.Gate
		if err := json.Unmarshal(event.Data, &gate); err != nil {
			return err
		}
		snapshot.Gates = replaceGate(snapshot.Gates, gate)
	case "pump.updated":
		var pump model.Pump
		if err := json.Unmarshal(event.Data, &pump); err != nil {
			return err
		}
		snapshot.Pumps = replacePump(snapshot.Pumps, pump)
	case "lease.updated":
		var lease model.CulvertLease
		if err := json.Unmarshal(event.Data, &lease); err != nil {
			return err
		}
		snapshot.Leases = replaceLease(snapshot.Leases, lease)
	case "forecast.updated":
		var forecast model.Forecast
		if err := json.Unmarshal(event.Data, &forecast); err != nil {
			return err
		}
		snapshot.Forecast = forecast
	case "permit.updated":
		var permit model.Permit
		if err := json.Unmarshal(event.Data, &permit); err != nil {
			return err
		}
		snapshot.Permit = permit
	case "incident.raised", "incident.resolved":
		var incident model.Incident
		if err := json.Unmarshal(event.Data, &incident); err != nil {
			return err
		}
		snapshot.Incidents = replaceIncident(snapshot.Incidents, incident)
	case "evidence.received":
		var evidence model.Evidence
		if err := json.Unmarshal(event.Data, &evidence); err != nil {
			return err
		}
		snapshot.Evidence = append(snapshot.Evidence, evidence)
	default:
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
	return nil
}

func replaceGate(items []model.Gate, value model.Gate) []model.Gate {
	result := append([]model.Gate(nil), items...)
	for index := range result {
		if result[index].ID == value.ID {
			result[index] = value
			return result
		}
	}
	return append(result, value)
}

func replacePump(items []model.Pump, value model.Pump) []model.Pump {
	result := append([]model.Pump(nil), items...)
	for index := range result {
		if result[index].ID == value.ID {
			result[index] = value
			return result
		}
	}
	return append(result, value)
}

func replaceLease(items []model.CulvertLease, value model.CulvertLease) []model.CulvertLease {
	result := append([]model.CulvertLease(nil), items...)
	for index := range result {
		if result[index].CulvertID == value.CulvertID {
			result[index] = value
			return result
		}
	}
	return append(result, value)
}

func replaceIncident(items []model.Incident, value model.Incident) []model.Incident {
	result := append([]model.Incident(nil), items...)
	for index := range result {
		if result[index].ID == value.ID {
			result[index] = value
			return result
		}
	}
	return append(result, value)
}
