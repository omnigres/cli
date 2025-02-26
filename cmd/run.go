package cmd

import (
	"context"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/omnigres/cli/orb"
	"github.com/omnigres/cli/src"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
)

var runCmd = &cobra.Command{
	Use:   "run [orb]",
	Short: "Run a one-off cluster",
	Long: `Run an Omnigres cluster in foreground.

    It's going to operate until it shut down. No run file will be created.
   `,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		var orbs []string
		if len(args) == 0 {
			var path string
			path, err = getOrbPath(false)

			orbs = []string{filepath.Base(path)}
		}

		if len(args) == 1 {
			inputPath := args[0]
			srcdir, err := src.GetSourceDirectory(inputPath)
			if err != nil {
				log.Fatal(err)
			}
			defer srcdir.Close()

			if src.IsGitHubGistURL(inputPath) {
				orbs = []string{"gist"}
			} else {
				workspace = srcdir.Path()
				var path string
				path, err = getOrbPath(false)

				orbs = []string{filepath.Base(path)}
			}

		}

		var cluster orb.OrbCluster
		cluster, err = getOrbCluster()

		if err != nil {
			log.Fatal(err)
		}

		cluster.Config().Image.Name = runImage

		ctx := context.Background()

		options := orb.OrbClusterStartOptions{
			Runfile:    false,
			AutoRemove: true,
			Listeners: []orb.OrbStartEventListener{{
				Ready: func(cluster orb.OrbCluster) {

					err := assembleOrbs(
						ctx,
						cluster,
						true,
						orbs,
						func(orbName string) string { return orbName },
					)

					if err != nil {
						log.Fatal(err)
					}

					if err != nil {
						log.Fatal(err)
					}

					var endpoints []orb.Endpoint
					endpoints, err = cluster.Endpoints(ctx)
					if err != nil {
						log.Fatal(err)
					}

					rows := make([][]string, 0)

					for _, endpoint := range endpoints {
						rows = append(rows, []string{endpoint.Database, endpoint.Protocol, endpoint.String()})
					}

					t := table.New().
						Border(lipgloss.RoundedBorder()).
						BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
						BorderColumn(false).
						Width(80).
						Headers("Orb", "Protocol", "URL").
						Rows(rows...)

					fmt.Println(t)

				},
			}},
		}
		options.Attachment.ShouldAttach = true
		options.Attachment.Listeners =
			[]orb.OrbRunEventListener{
				{
					OutputHandler: func(cluster orb.OrbCluster, reader io.Reader) {
						go func() { _, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, reader) }()
					},
				},
			}
		err = cluster.Start(ctx, options, nil, nil)

		if err != nil {
			log.Fatal(err)
		}
	},
}

var runImage string

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runImage, "image", "i", orb.NewConfig().Image.Name, "The Omnigres image to use")
}
