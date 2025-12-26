package commands

// Command to run holders dynamics monitor in standalone mode
// Runs only the token holder changes tracking monitor
// Tracks investments, sales, and liquidations
// Implements graceful shutdown for proper termination

import (
	"context"
	"os"
	"os/signal"
	"spark-wallet/bots_monitor"
	"spark-wallet/internal/infra/log"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var holdersCmd = &cobra.Command{
	Use:   "holders",
	Short: "Run Holders dynamics monitor standalone",
	Long:  `Run only the Holders Dynamic monitor to track token holder changes, investments, and liquidations.`,
	RunE:  runHolders,
}

func runHolders(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		bots_monitor.RunHoldersDynamicMonitor()
	}()

	log.LogSuccess("Holders monitor is running", zap.String("status", "active"))

	<-ctx.Done()
	log.LogInfo("Shutdown signal received, gracefully stopping...")
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.LogSuccess("Holders monitor stopped gracefully")
	case <-time.After(10 * time.Second):
		log.LogWarn("Timeout waiting for monitor to stop")
	}

	return nil
}
