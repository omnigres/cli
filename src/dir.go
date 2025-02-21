package src

import (
	"os"

	"github.com/charmbracelet/log"
)

func IsDirectory(input string) bool {
	info, err := os.Stat(input)
	log.Debug("IsDirectory", "input", input, "info", info, "err", err)
	return err == nil && info.IsDir()
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
