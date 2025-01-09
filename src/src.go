package src

import (
	"fmt"
	"io"
)

type SourceDirectory interface {
	Path() string
	io.Closer
}

func GetSourceDirectory(input string) (src SourceDirectory, err error) {
	if IsGitHubGistURL(input) {
		return getGitHubGist(input)
	} else if IsDirectory(input) {
		src = &ExistingDirectory{directory: input}
	} else {
		err = fmt.Errorf("Invalid source `%s`. Is this a valid, existing directory?", input)
	}
	return
}
