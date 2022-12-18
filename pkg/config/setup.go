package config

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

/*
	The goal here is to reduce _undocumented global state_ and odd behavior handling it.
	Things like env vars and pwd can change during runtime.
	This is a highly probable source of undefined or confusing behavior.
	It may even force strange behavior such as moving directories temporarily
	before running an operation, which would preclude concurrent operations.
	This may not be the _most correct_ or _best_ way to handle these issues.
	But it is certainly a step forward.
	This doesn't prevent changes that might be passed down to sub-procsses like rio, runc
*/

type State struct {
	Env              map[string]string
	HomeDirectory    string
	WorkingDirectory string
	ExecutablePath   string
	TempDir          string
}

var (
	globalm sync.RWMutex
	global  State
)

// ReloadGlobalState will fetch all values for internal state
// ReloadGlobalState will halt on the first error.
// ReloadGlobalState is as concurrent safe as we can manage.
//
// Errors:
//
//   - warpforge-error-initialization -- loading the value failed
func ReloadGlobalState() error {
	globalm.Lock()
	defer globalm.Unlock()
	global.Env = make(map[string]string, len(envKeys))
	for _, key := range envKeys {
		if v, ok := os.LookupEnv(key); ok {
			global.Env[key] = v
		}
	}
	loadFuncs := []func() error{
		loadExecutablePath,
		loadWd,
		loadUserHome,
		loadTempDir,
	}
	for _, loadFunc := range loadFuncs {
		if err := loadFunc(); err != nil {
			// Error Codes = warpforge-error-initialization
			return err
		}
	}
	return nil
}

// NewState will create a copy of the global state.
// The returned state can be modified without affecting anything else.
// ReloadState will not affect this copy
// NewState is concurrent safe.
//
// Errors:
//
//   - warpforge-error-serialization -- error copying data
func NewState() (State, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	enc := json.NewEncoder(buf)
	dec := json.NewDecoder(buf)
	var result State
	// most memory allocations can occur before the lock
	globalm.RLock()
	defer globalm.RUnlock()
	err := enc.Encode(global)
	if err != nil {
		return State{}, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err))
	}
	err = dec.Decode(&result)
	if err != nil {
		return State{}, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err))
	}
	return result, nil
}

// init will load all guarded values and will terminate execution if an error occurs.
func init() {
	if err := ReloadGlobalState(); err != nil {
		serr, ok := err.(serum.ErrorInterface)
		if !ok {
			serr = serum.Error(wfapi.ECodeUnknown,
				serum.WithMessageLiteral("config initialization failed"),
				serum.WithCause(err),
			).(serum.ErrorInterface)
		}
		wfapi.TerminalError(serr, 10)
	}
}

// loadExecutablePath stores the path to the executable into the stored state
// NOT concurrent safe
//
// Errors:
//
//    - warpforge-error-initialization -- when the path to the warpforge executable cannot be found
func loadExecutablePath() error {
	path, err := os.Executable()
	if err != nil {
		return serum.Error(wfapi.ECodeInitialization,
			serum.WithMessageLiteral("failed to locate binary path"),
			serum.WithCause(err),
		)
	}
	global.ExecutablePath = path
	return nil
}

// loadWd loads the working directory into the stored state
// NOT concurrent safe
//
// Errors:
//
//    - warpforge-error-initialization -- when the working directory path cannot be found
func loadWd() error {
	cwd, err := os.Getwd()
	if err != nil {
		return serum.Error(wfapi.ECodeInitialization,
			serum.WithMessageLiteral("unable to get working directory"),
			serum.WithCause(err),
		)
	}
	global.WorkingDirectory = cwd
	return nil
}

// loadUserHome loads user home directory into the stored state
// NOT concurrent safe
//
// Errors:
//
//    - warpforge-error-initialization -- when the user home directory path cannot be found
func loadUserHome() error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return serum.Error(wfapi.ECodeInitialization,
			serum.WithMessageLiteral("unable to find user home directory"),
			serum.WithCause(err),
		)
	}
	global.HomeDirectory = dir
	return nil
}

// loadTempDir loads the defualt temporary file directory into stored state
// NOT concurrent safe
func loadTempDir() error {
	global.TempDir = os.TempDir()
	return nil
}
