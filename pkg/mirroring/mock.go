package mirroring

import "github.com/warptools/warpforge/wfapi"

// A fake pusher that is intended for tests only. This will do nothing when "pushing" wares other than
// keep track of the wares that have been pushed.

type MockPusher struct {
	cfg   wfapi.MockPushConfig
	wares map[wfapi.WareID]bool
}

func NewMockPusher(cfg wfapi.MockPushConfig) (MockPusher, error) {
	return MockPusher{cfg: cfg}, nil
}

func (p *MockPusher) hasWare(wareId wfapi.WareID) (bool, error) {
	_, exists := p.wares[wareId]
	return exists, nil
}

func (p *MockPusher) pushWare(wareId wfapi.WareID, localPath string) error {
	p.wares[wareId] = true
	return nil
}
