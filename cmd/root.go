package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"os"
	"path/filepath"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "omnigres",
	Short: "Omnigres CLI",
	Long:  `Omnigres CLI toolkit allows easy creation and management of Omnigres applications and orbs.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			log.SetLevel(log.DebugLevel)
			log.Debug("Verbose mode enabled")
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var workspace string
var verbose bool

func findOmnigresDir(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "omnigres.yaml")); err == nil {
		return dir, nil
	}

	parent := filepath.Dir(dir)
	if parent == dir {
		// Reached the root, return original working directory
		return os.Getwd()
	}
	return findOmnigresDir(parent)
}

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	omnigresDir, err := findOmnigresDir(cwd)
	if err != nil {
		log.Fatal(err)
	}
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", omnigresDir, "path to workspace")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "display debug messages")
}
