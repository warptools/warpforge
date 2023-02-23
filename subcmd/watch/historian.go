package watch

import (
	"context"
	"sync"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

// Historian is a minimal placeholder for an actor tracking module state.
type historian struct {
	m       sync.RWMutex // Locking strategy is a simple global lock for now. Reconsider this strategy over time.
	records map[string]*moduleHistory
}

// ModuleHistory contains data required for history of a single module state
type moduleHistory struct {
	// we likely want to store more records and information.
	recent workspaceapi.ModuleStatus
}

// setStatus will set the current state of a module
func (h *historian) setStatus(path string, ingests map[string]string, status workspaceapi.ModuleStatus) {
	if h == nil {
		return
	}
	h.m.Lock()
	defer h.m.Unlock()
	if h.records == nil {
		h.records = make(map[string]*moduleHistory)
	}
	record, ok := h.records[path]
	if !ok || record == nil {
		record = &moduleHistory{}
		h.records[path] = record
	}
	record.recent = status
}

// getStatus will retrieve the most recent state of a module
func (h *historian) getStatus(ctx context.Context, path string) (workspaceapi.ModuleStatus, error) {
	if h == nil {
		return workspaceapi.ModuleStatus_NoInfo, serum.Error(wfapi.ECodeInternal, serum.WithMessageLiteral("historian not provisioned"))
	}
	h.m.RLock()
	defer h.m.RUnlock()
	record, ok := h.records[path]
	if !ok {
		return workspaceapi.ModuleStatus_NoInfo, nil
	}
	return record.recent, nil
}
