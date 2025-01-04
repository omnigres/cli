package cmd

import (
	"context"
	"fmt"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var endpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Print cluster endpoints",
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		var endpoints []orb.Endpoint
		endpoints, err = cluster.Endpoints(ctx)
		if err != nil {
			panic(err)
		}

		for _, endpoint := range endpoints {
			fmt.Printf("%s (%s): %s\n", endpoint.Database, endpoint.Protocol, endpoint.String())
		}

	},
}

func init() {
	rootCmd.AddCommand(endpointsCmd)
}
