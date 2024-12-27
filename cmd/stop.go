package cmd

import (
	"context"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop running cluster",
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		err = cluster.Stop(ctx)
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
