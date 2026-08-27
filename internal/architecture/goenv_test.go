package architecture

import (
	"os"
	"path/filepath"
	"runtime"
)

// goModContent returns this module's go.mod file content, resolved relative
// to this source file's own location so `go test ./...` works regardless of
// the caller's working directory.
func goModContent() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	// this file: internal/architecture/goenv.go -> module root is two dirs up.
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
