package cmd

import (
	"context"
	"fmt"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start cluster",
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		err = cluster.Start(ctx)
		if err != nil {
			panic(err)
		}
		err = cluster.Config().Save()
		if err != nil {
			panic(err)
		}

		fmt.Println("Omnigres Orb cluster started.\n")

		http_port, err := cluster.Port(ctx, "8080/tcp")
		if err != nil {
			panic(err)
		}
		fmt.Printf("HTTP: http://127.0.0.1:%d\n", http_port)
		pg_port, err := cluster.Port(ctx, "5432/tcp")
		if err != nil {
			panic(err)
		}
		fmt.Printf("Postgres: postgres://omnigres:omnigres@127.0.0.1:%d/omnigres\n", pg_port)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
