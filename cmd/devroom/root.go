package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X main.Version=<ver>".
var Version = "dev"

var showVersion bool

var rootCmd = &cobra.Command{
	Use:   "devroom",
	Short: "Repo-aware Claude workspace launcher",
	Long:  `devroom manages persistent containerised development environments tied to feature branches.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Printf("devroom %s\n", Version)
			return nil
		}
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&showVersion, "version", "V", false, "Print version and exit")
}
