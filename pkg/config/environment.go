package config

const (
	// EnvWarpforgeKeepRundir can be set to prevent removing the created temporary directory after execution
	EnvWarpforgeKeepRundir = "WARPFORGE_KEEP_RUNDIR"
	// EnvWarpforgePath is the path to expected plugins (rio, runc)
	EnvWarpforgePath = "WARPFORGE_PATH"
	// EnvWarpforgeRunPath can be set to override the path where warpforge will store temporary files used for formula execution
	// Warpforge run will create new temporary directories here.
	EnvWarpforgeRunPath = "WARPFORGE_RUNPATH"
	// EnvWarpforgeWarehouse will override the warehouse used for execution
	EnvWarpforgeWarehouse = "WARPFORGE_WAREHOUSE"
)

// NOTE: keep this up to date or the config loader won't load them
var envKeys = []string{
	EnvWarpforgeKeepRundir,
	EnvWarpforgePath,
	EnvWarpforgeRunPath,
	EnvWarpforgeWarehouse,
}
