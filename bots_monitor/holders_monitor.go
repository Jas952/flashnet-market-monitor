package bot

import (
	"spark-wallet/internal/infra/log"
	"time"

	"go.uber.org/zap"
)

func RunHoldersMonitor(checkInterval time.Duration) {
	log.LogWarn("RunHoldersMonitor is deprecated. Use RunHoldersDynamicMonitor instead.")
	log.LogInfo("Starting Holders Monitor (deprecated - no action will be performed)...")

	log.LogSuccess("Holders monitor (deprecated) is running", zap.String("status", "active"))
	select {}
}
