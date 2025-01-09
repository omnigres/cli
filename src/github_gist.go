package src

import (
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
	response, err := http.Get(input)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		println("status code error: %d %s", response.StatusCode, response.Status)
		return
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "omnigres-gist")
	if err != nil {
		panic(err)
	}
	//defer os.RemoveAll(dir)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if ok, _ := regexp.MatchString("/raw/", href); ok {
			response, err := http.Get("https://gist.github.com" + href)
			if err != nil {
				panic(err)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				println("status code error: %d %s", response.StatusCode, response.Status)
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
