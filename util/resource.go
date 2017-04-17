package util

import (
	"path/filepath"
	"runtime"
)

var baseResourcePath string

func init() {
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to determine base resource path")
	}

	baseResourcePath, _ = filepath.Split(path)
	baseResourcePath, _ = filepath.Split(filepath.Clean(baseResourcePath))
}

func ResourcePath(relativePath string) string {
	return filepath.Join(baseResourcePath, relativePath)
}
