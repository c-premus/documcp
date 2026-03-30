package main

import (
	"fmt"
	"os"
)

// Set via ldflags at build time.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
