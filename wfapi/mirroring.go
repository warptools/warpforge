package wfapi

type MirroringConfig struct {
	Keys   []WarehouseAddr
	Values map[WarehouseAddr]WarehouseMirroringConfig
}

type WarehouseMirroringConfig struct {
	PushConfig WarehousePushConfig
}

type WarehousePushConfig struct {
	S3   *S3PushConfig
	Mock *MockPushConfig
}

type S3PushConfig struct {
	Endpoint string
	Region   string
	Bucket   string
	Path     *string
}

type MockPushConfig struct {
}
