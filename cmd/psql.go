package cmd

import (
	"context"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var database string = "omnigres"

var psqlCmd = &cobra.Command{
	Use:   "psql",
	Short: "Connect to the cluster using psql",
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		err = cluster.ConnectPsql(ctx, database)
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(psqlCmd)
	psqlCmd.Flags().StringVarP(&database, "database", "d", "omnigres", "Database to connect to")
}
