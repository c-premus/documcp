package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("documcp %s (built %s)\n", version, buildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
