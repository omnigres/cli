/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"github.com/omnigres/cli/diff"
	"github.com/omnigres/cli/orb"
	"github.com/samber/lo"

	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		orbs := lo.Map(cluster.Config().Orbs, func(cfg orb.OrbCfg, _ int) string { return cfg.Name })
		orbs_ := lo.Map(cluster.Config().Orbs, func(cfg orb.OrbCfg, _ int) string { return cfg.Name + "_diff" })
		err = migrate(ctx, cluster, true, orbs, orbs_)
		if err != nil {
			panic(err)
		}
		switch diffMethod {
		case "migra":
			for i, orb := range orbs {
				err = diff.Migra(ctx, cluster, orb, orbs_[i])
				if err != nil {
					panic(err)
				}
			}
		default:
			println("Unknown diff method: " + diffMethod)
			return
		}
		if err != nil {
			panic(err)
		}
	},
}

var diffMethod string

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&diffMethod, "method", "m", "", "Diff method")
	if diffCmd.MarkFlagRequired("method") != nil {
		panic("can't make flag required for diff")
	}
}
