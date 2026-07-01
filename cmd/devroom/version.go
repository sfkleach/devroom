package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the devroom version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("devroom %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
