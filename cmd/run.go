package cmd

import (
	"context"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/omnigres/cli/orb"
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
		var orbs, databases []string
		if len(args) == 0 {
			var path string
			path, err = getOrbPath(false)

			orbs = []string{filepath.Base(path)}
			databases = orbs
		}
		if len(args) == 1 {
			path := args[0]

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
			databases = []string{filepath.Base(path)}
		}

		var cluster orb.OrbCluster
		cluster, err = getOrbCluster()

		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		err = cluster.Run(ctx, orb.RunEventListener{
			OutputHandler: func(cluster orb.OrbCluster, reader io.Reader) {
				go func() { _, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, reader) }()
			},
			Ready: func(cluster orb.OrbCluster) {

				fmt.Println("Starting migration...")
				err := migrate(ctx, cluster, true, orbs, databases)

				if err != nil {
					panic(err)
				}

				http_port, err := cluster.Port(ctx, "8080/tcp")
				if err != nil {
					panic(err)
				}
				pg_port, err := cluster.Port(ctx, "5432/tcp")
				if err != nil {
					panic(err)
				}

				rows := [][]string{
					{""},
					{fmt.Sprintf("HTTP: http://127.0.0.1:%d", http_port)},
					{fmt.Sprintf("Postgres: postgres://omnigres:omnigres@127.0.0.1:%d/omnigres", pg_port)},
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

func init() {
	rootCmd.AddCommand(runCmd)
}
