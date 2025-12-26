package main

// Main entry point of the application
// Initializes and executes Cobra commands
// Handles command execution errors

import (
	"fmt"
	"os"

	"spark-wallet/cmd/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
