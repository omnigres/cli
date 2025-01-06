package cmd

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var runCmd = &cobra.Command{
	Use:   "run [orb]",
	Short: "Run a one-off cluster",
	Long: `Run an Omnigres cluster in foreground.

    It's going to operate until it shut down. No run file will be created.
   `,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		var orbs, databases []string
		if len(args) == 0 {
			var path string
			path, err = getOrbPath(false)

			orbs = []string{filepath.Base(path)}
			databases = orbs
		}
		if len(args) == 1 {
			path := args[0]
			if ok, _ := regexp.MatchString("https://gist.github.com/(.+)/(.+)", path); ok {
				response, err := http.Get(path)
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
				dir, err := os.MkdirTemp("", "omnigres-run")
				if err != nil {
					panic(err)
				}
				defer os.RemoveAll(dir)

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
						file, err := os.Create(filepath.Join(dir, filename))
						if err != nil {
							panic(err)
						}
						defer file.Close()
						_, err = io.Copy(file, response.Body)

						if err != nil {
							log.Fatalf("Failed to copy data to file: %v", err)
						}
					}
				})
				path = dir
				databases = []string{"gist"}
			}

			workspace = path
			path, err = getOrbPath(false)

			if err != nil {
				panic(err)
			}

			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				fmt.Printf("Path does not exist: %s\n", path)
				return
			} else if err != nil {
				fmt.Printf("Error checking path: %v\n", err)
				return
			}

			if !info.IsDir() {
				fmt.Printf("Path exists but is not a directory: %s\n", path)
				return
			}
			orbs = []string{"."}
			if len(databases) == 0 {
				databases = []string{filepath.Base(path)}
			}
		}

		var cluster orb.OrbCluster
		cluster, err = getOrbCluster()
		cluster.Config().Image.Name = runImage

		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		err = cluster.Run(ctx, orb.OrbRunEventListener{
			OutputHandler: func(cluster orb.OrbCluster, reader io.Reader) {
				go func() { _, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, reader) }()
			},
			Ready: func(cluster orb.OrbCluster) {

				err := migrate(ctx, cluster, true, orbs, databases)

				if err != nil {
					panic(err)
				}

				if err != nil {
					panic(err)
				}

				var endpoints []orb.Endpoint
				endpoints, err = cluster.Endpoints(ctx)
				if err != nil {
					panic(err)
				}

				rows := [][]string{
					{""},
				}

				for _, endpoint := range endpoints {
					rows = append(rows, []string{fmt.Sprintf("%s (%s): %s", endpoint.Database, endpoint.Protocol, endpoint.String())})
				}

				t := table.New().
					Border(lipgloss.RoundedBorder()).
					BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
					Headers("Omnigres Cluster").
					Rows(rows...)

				fmt.Println(t)

			},
		})
		if err != nil {
			panic(err)
		}

	},
}

var runImage string

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runImage, "image", "i", orb.NewConfig().Image.Name, "The Omnigres image to use")
}
