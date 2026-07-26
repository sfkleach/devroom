package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X main.Version=<ver>".
var Version = "dev"

var showVersion bool
var rootDir string

var rootCmd = &cobra.Command{
	Use:   "devroom",
	Short: "Repo-aware Claude workspace launcher",
	Long:  `devroom manages persistent containerised development environments tied to feature branches.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Printf("devroom %s\n", Version)
			return nil
		}
		return runTUI()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// effectiveRootDir returns --rootdir if set, otherwise the current working directory.
func effectiveRootDir() (string, error) {
	if rootDir != "" {
		return rootDir, nil
	}
	return os.Getwd()
}

func init() {
	rootCmd.Flags().BoolVarP(&showVersion, "version", "V", false, "Print version and exit")
	rootCmd.PersistentFlags().StringVarP(&rootDir, "rootdir", "r", "", "Project root directory (default: current working directory)")
}
