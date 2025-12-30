package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RunNodeScript executes a Node.js script with validation and timeout
// Returns combined output and error
func RunNodeScript(scriptPath string, timeout time.Duration, args ...string) ([]byte, error) {
	// Validate Node.js is installed
	if err := validateNodeInstalled(); err != nil {
		return nil, err
	}

	// Validate script exists
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve script path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("script not found: %s", absPath)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build command
	cmdArgs := append([]string{absPath}, args...)
	cmd := exec.CommandContext(ctx, "node", cmdArgs...)
	cmd.Dir = "."

	// Execute with timeout
	output, err := cmd.CombinedOutput()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %v", timeout)
	}

	return output, err
}

// validateNodeInstalled checks if Node.js is available
func validateNodeInstalled() error {
	cmd := exec.Command("node", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Node.js is not installed or not in PATH: %w", err)
	}
	return nil
}
