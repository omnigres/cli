package src

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

func IsGitHubGistURL(input string) bool {
	ok, _ := regexp.MatchString("https://gist.github.com/(.+)/(.+)", input)
	return ok
}

type tempGistDirectory struct {
	path string
}

func (t *tempGistDirectory) Path() string {
	return t.path
}

func (t *tempGistDirectory) Close() error {
	return os.RemoveAll(t.path)
}

func getGitHubGist(input string) (srcdir SourceDirectory, err error) {
	var response *http.Response
	response, err = http.Get(input)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		err = fmt.Errorf("status code error: %d %s", response.StatusCode, response.Status)
		return
	}
	var doc *goquery.Document
	doc, err = goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return
	}
	var dir string
	dir, err = os.MkdirTemp("", "omnigres-gist")
	if err != nil {
		return
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if ok, _ := regexp.MatchString("/raw/", href); ok {
			response, err = http.Get("https://gist.github.com" + href)
			if err != nil {
				return
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				err = fmt.Errorf("status code error: %d %s", response.StatusCode, response.Status)
				return
			}
			filename := filepath.Base(href)
			var file *os.File

			file, err = os.Create(filepath.Join(dir, filename))
			if err != nil {
				return
			}
			defer file.Close()
			_, err = io.Copy(file, response.Body)

			if err != nil {
				return
			}
		}
	})
	srcdir = &tempGistDirectory{path: dir}
	return
}
