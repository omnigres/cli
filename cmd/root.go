package cmd

import (
	"github.com/charmbracelet/log"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "omnigres",
	Short: "Omnigres CLI",
	Long:  `Omnigres CLI toolkit allows easy creation and management of Omnigres applications and orbs.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var workspace string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", cwd, "path to workspace")
}
