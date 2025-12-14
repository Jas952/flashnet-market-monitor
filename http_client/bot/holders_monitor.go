package bot

import (
	"spark-wallet/http_client/system_works"
	"time"

	"go.uber.org/zap"
)

func RunHoldersMonitor(checkInterval time.Duration) {
	system_works.LogWarn("RunHoldersMonitor is deprecated. Use RunHoldersDynamicMonitor instead.")
	system_works.LogInfo("Starting Holders Monitor (deprecated - no action will be performed)...")

	system_works.LogSuccess("Holders monitor (deprecated) is running", zap.String("status", "active"))
	select {}
}
