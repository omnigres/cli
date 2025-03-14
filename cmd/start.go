package cmd

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
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
			log.Fatal(err)
		}

		ctx := context.Background()

		readyCh := make(chan orb.OrbCluster, 1)
		err = cluster.StartWithCurrentUser(ctx, orb.OrbClusterStartOptions{Runfile: true, Listeners: []orb.OrbStartEventListener{
			{Ready: func(cluster orb.OrbCluster) {
				readyCh <- cluster
			}}}})
		if err != nil {
			log.Fatal(err)
		}
		err = cluster.Config().Save()
		if err != nil {
			log.Fatal(err)
		}

		<-readyCh

		log.Info("Omnigres Orb cluster started.")

		var endpoints []orb.Endpoint
		endpoints, err = cluster.Endpoints(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, endpoint := range endpoints {
			fmt.Printf("%s (%s): %s\n", endpoint.Database, endpoint.Protocol, endpoint.String())
		}

	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
