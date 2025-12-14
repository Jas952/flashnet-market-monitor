package main

// in Go file (package)
// package main -
// Go main() in main

import (
	"spark-wallet/http_client/bot"
	"spark-wallet/http_client/system_works"
	"time"

	"go.uber.org/zap"
)

// func main() -
func main() {
	// Start
	// Check 1 by default
	// in Telegram -
	checkInterval := 1 * time.Hour
	bot.RunHoldersMonitor(checkInterval)

	system_works.LogSuccess("Holders monitor is running", zap.String("status", "active"))
	select {}
}
