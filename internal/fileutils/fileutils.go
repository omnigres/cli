package fileutils

import (
	"os"
	"path/filepath"
)

func CreateIfNotExists(path string, isDirectory bool) (err error) {
	if isDirectory {
		err = os.MkdirAll(path, 0o755)
		return
	} else {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err != nil {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	return
}
