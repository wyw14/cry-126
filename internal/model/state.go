package model

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

type LeaseDirection string

const (
	DirectionInward  LeaseDirection = "inward"
	DirectionOutward LeaseDirection = "outward"
)

type CulvertLease struct {
	ID                  uuid.UUID      `json:"id"`
	CulvertID           string         `json:"culvert_id"`
	Owner               string         `json:"owner"`
	Direction           LeaseDirection `json:"direction"`
	OperationID         uuid.UUID      `json:"operation_id"`
	OperationGeneration uint64         `json:"operation_generation"`
	FencingToken        uint64         `json:"fencing_token"`
	AcquiredAt          time.Time      `json:"acquired_at"`
	ExpiresAt           time.Time      `json:"expires_at"`
}

type IncidentSeverity string

const (
	SeverityInfo     IncidentSeverity = "info"
	SeverityCritical IncidentSeverity = "critical"
)

type Incident struct {
	ID                  uuid.UUID        `json:"id"`
	OperationID         uuid.UUID        `json:"operation_id"`
	OperationGeneration uint64           `json:"operation_generation"`
	Severity            IncidentSeverity `json:"severity"`
	Code                string           `json:"code"`
	Message             string           `json:"message"`
	RaisedAt            time.Time        `json:"raised_at"`
	ResolvedAt          *time.Time       `json:"resolved_at,omitempty"`
}

type SystemSnapshot struct {
	Operation Operation      `json:"operation"`
	Forecast  Forecast       `json:"forecast"`
	Permit    Permit         `json:"permit"`
	Gates     []Gate         `json:"gates"`
	Pumps     []Pump         `json:"pumps"`
	Leases    []CulvertLease `json:"leases"`
	Evidence  []Evidence     `json:"evidence"`
	Incidents []Incident     `json:"incidents"`
	Cursor    uint64         `json:"cursor"`
	SavedAt   time.Time      `json:"saved_at"`
}

func (lease CulvertLease) Validate() error {
	if lease.ID == uuid.Nil || lease.OperationID == uuid.Nil {
		return errors.New("lease identities are required")
	}
	if lease.CulvertID == "" || lease.Owner == "" {
		return errors.New("lease culvert and owner are required")
	}
	if lease.Direction != DirectionInward && lease.Direction != DirectionOutward {
		return errors.New("lease direction is invalid")
	}
	if lease.FencingToken == 0 || lease.OperationGeneration == 0 {
		return errors.New("lease generations are required")
	}
	if !lease.ExpiresAt.After(lease.AcquiredAt) {
		return errors.New("lease expiry must follow acquisition")
	}
	return nil
}

func (incident Incident) Active() bool {
	return incident.ResolvedAt == nil
}

func CloneSnapshot(source SystemSnapshot) SystemSnapshot {
	result := source
	result.Gates = append([]Gate(nil), source.Gates...)
	result.Pumps = append([]Pump(nil), source.Pumps...)
	result.Leases = append([]CulvertLease(nil), source.Leases...)
	result.Evidence = append([]Evidence(nil), source.Evidence...)
	result.Incidents = append([]Incident(nil), source.Incidents...)
	return result
}

func SortSnapshot(snapshot *SystemSnapshot) {
	sort.Slice(snapshot.Gates, func(i, j int) bool { return snapshot.Gates[i].ID < snapshot.Gates[j].ID })
	sort.Slice(snapshot.Pumps, func(i, j int) bool { return snapshot.Pumps[i].ID < snapshot.Pumps[j].ID })
	sort.Slice(snapshot.Leases, func(i, j int) bool { return snapshot.Leases[i].CulvertID < snapshot.Leases[j].CulvertID })
	sort.Slice(snapshot.Incidents, func(i, j int) bool { return snapshot.Incidents[i].RaisedAt.Before(snapshot.Incidents[j].RaisedAt) })
	snapshot.Evidence = SortedEvidence(snapshot.Evidence)
}

func ActiveIncidents(items []Incident) []Incident {
	result := make([]Incident, 0, len(items))
	for _, item := range items {
		if item.Active() {
			result = append(result, item)
		}
	}
	return result
}
