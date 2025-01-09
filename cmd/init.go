package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/omnigres/cli/internal/fileutils"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
	"path/filepath"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new cluster",
	Long: `Initializes a new Omnigres cluster.

Omnigres projects follow architecture and guidelines set out in Omnigres
design and use one or more Omnigres extension.`,

	Run: func(cmd *cobra.Command, args []string) {
		var path string
		var err error
		path, err = getOrbPath(true)
		if err != nil {
			log.Fatal(err)
		}
		var orbName string
		if len(args) > 0 {
			orbName = args[0]
			path = filepath.Join(path, orbName)
		} else {
			orbName = filepath.Base(path)
		}

		cfg := orb.NewConfig()
		cfg.Orbs = append(cfg.Orbs, orb.OrbCfg{
			Name: orbName,
		})
		err = cfg.SaveAs(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, dir := range []string{"src", "migrations"} {
			err = fileutils.CreateIfNotExists(filepath.Join(path, dir), true)
			if err != nil {
				log.Fatal(err)
			}
		}
	},
	Args: cobra.MaximumNArgs(1),
}

func init() {
	rootCmd.AddCommand(initCmd)
}
