package culvert

import (
	"errors"

	"github.com/wyw14/cry-126/internal/model"
)

type Recovery struct {
	table  *LeaseTable
	loaded bool
}

func NewRecovery(table *LeaseTable) (*Recovery, error) {
	if table == nil {
		return nil, errors.New("culvert lease table is required")
	}
	return &Recovery{table: table}, nil
}

func (recovery *Recovery) Load(snapshot model.SystemSnapshot) error {
	if err := recovery.table.Restore(snapshot.Leases); err != nil {
		return err
	}
	recovery.loaded = true
	return nil
}

func (recovery *Recovery) Loaded() bool {
	return recovery.loaded
}

func (recovery *Recovery) RequireLoaded() error {
	if !recovery.loaded {
		return errors.New("culvert ownership and fencing are not loaded")
	}
	return nil
}

func (recovery *Recovery) Leases() ([]model.CulvertLease, error) {
	if err := recovery.RequireLoaded(); err != nil {
		return nil, err
	}
	return recovery.table.Snapshot(), nil
}
