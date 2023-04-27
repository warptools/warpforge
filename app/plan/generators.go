package plancli

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/warptools/warpforge/wfapi"
)

// map of plot filenames to the function used for generation
var generators = map[string]func(string) ([]byte, error){
	"plot.star": starlarkGenerator,
}

// handle starlark plots (plot.star)
// Errors:
//
//    - warpforge-error-generator-failed -- when the external generator fails
func starlarkGenerator(file string) ([]byte, error) {
	cmd := exec.Command("warplark", file)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return []byte{}, wfapi.ErrorGeneratorFailed("warplark", file, "\n"+string(exitErr.Stderr))
		} else {
			return []byte{}, wfapi.ErrorGeneratorFailed("warplark", file, "failed to run warplark")
		}
	}
	return out, nil
}

// GenerateFile takes a path to a file and runs the corresponding generator.
// Errors:
//
//    - warpforge-error-generator-failed -- when the external generator fails
func GenerateFile(path string) ([]byte, error) {
	for fname, generatorFunc := range generators {
		if fname == filepath.Base(path) {
			// Error Codes = warpforge-error-generator-failed
			return generatorFunc(path)
		}
	}
	return nil, nil
}

// GenerateDir takes a path to a directory and runs the corresponding
// generators for all generatable files in the directory
// Errors:
//
//    - warpforge-error-generator-failed -- when the external generator fails
func GenerateDir(path string) (map[string][]byte, error) {
	results := map[string][]byte{}
	for fname, generatorFunc := range generators {
		file := filepath.Join(path, fname)
		if _, err := os.Stat(file); err == nil {
			out, err := generatorFunc(file)
			if err != nil {
				// Error Codes = warpforge-error-generator-failed
				return map[string][]byte{}, err
			}
			results[file] = out
		}
	}
	return results, nil
}

// GenerateDirRecursive takes a path to a directory and runs the corresponding
// generators for all generatable files in the directory, recursing into
// subdirectories
// Errors:
//
//    - warpforge-error-generator-failed -- when the external generator fails
//    - warpforge-error-io -- when the dir walk fails
func GenerateDirRecusive(startPath string) (map[string][]byte, error) {
	results := map[string][]byte{}
	err := filepath.Walk(startPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return wfapi.ErrorIo("walking directory", path, err)
			}
			if genFunc, exists := generators[filepath.Base(path)]; exists {
				out, err := genFunc(path)
				if err != nil {
					return err
				}
				results[path] = out
			}
			return nil
		})

	// Error Codes = warpforge-error-generator-failed, warpforge-error-io
	return results, err
}
