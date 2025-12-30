package fs

import (
	"fmt"
	"os"
	"time"
)

// WaitForFile waits for a file to exist and be non-empty with exponential backoff
// Returns error if file doesn't appear within maxWait duration
func WaitForFile(filePath string, maxWait time.Duration) error {
	start := time.Now()
	attempt := 0
	baseDelay := 50 * time.Millisecond

	for {
		// Check if file exists and is non-empty
		if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
			return nil
		}

		// Check if we've exceeded max wait time
		if time.Since(start) >= maxWait {
			return fmt.Errorf("timeout waiting for file %s after %v", filePath, maxWait)
		}

		// Exponential backoff with cap at 500ms
		delay := baseDelay * time.Duration(1<<attempt)
		if delay > 500*time.Millisecond {
			delay = 500 * time.Millisecond
		}

		time.Sleep(delay)
		attempt++
	}
}
