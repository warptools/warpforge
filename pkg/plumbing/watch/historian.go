package watch

import (
	"context"
	"sync"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

type historian struct {
	m       sync.RWMutex
	records map[string]*moduleHistory
}
type moduleHistory struct {
	// we likely want to store more records and information.
	recent workspaceapi.ModuleStatus
}

func (h *historian) SetStatus(path string, ingests map[string]string, status workspaceapi.ModuleStatus) {
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

func (h *historian) GetStatus(ctx context.Context, path string) (workspaceapi.ModuleStatus, error) {
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
