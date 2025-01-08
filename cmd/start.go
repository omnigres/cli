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

		readyCh := make(chan orb.OrbCluster, 1)
		err = cluster.Start(ctx, orb.OrbClusterStartOptions{Runfile: true, Listeners: []orb.OrbStartEventListener{
			{Ready: func(cluster orb.OrbCluster) {
				readyCh <- cluster
			}}}})
		if err != nil {
			panic(err)
		}
		err = cluster.Config().Save()
		if err != nil {
			panic(err)
		}

		<-readyCh

		fmt.Println("Omnigres Orb cluster started.\n")

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
	rootCmd.AddCommand(startCmd)
}
