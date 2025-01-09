package src

import (
	"os"
)

func IsDirectory(input string) bool {

	info, err := os.Stat(input)

	if err != nil {
		return false
	}

	return info.IsDir()
}

type ExistingDirectory struct {
	directory string
}

func (s *ExistingDirectory) Path() string {
	return s.directory
}

func (s *ExistingDirectory) Close() error {
	return nil
}
