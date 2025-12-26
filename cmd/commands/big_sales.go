package commands

// Command to run Big Sales monitor in standalone mode
// Runs only the large purchases and sales monitor without other features
// Useful for testing or dedicated monitoring
// Implements graceful shutdown for proper termination

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"spark-wallet/bots_monitor"
	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/infra/log"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var bigSalesCmd = &cobra.Command{
	Use:   "big-sales",
	Short: "Run Big Sales monitor standalone",
	Long:  `Run only the Big Sales monitor without other features. Useful for testing or dedicated monitoring.`,
	RunE:  runBigSales,
}

func runBigSales(cmd *cobra.Command, args []string) error {
	godotenv.Load(".env")

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	publicKey := os.Getenv("PUBLIC_KEY")
	dataDir := "data_in"

	log.LogInfo("Starting Big Sales Monitor...")
	log.LogInfo("Network", zap.String("network", network))

	client := flashnet.NewAMMClient(network)

	if publicKey != "" {
		if err := ensureValidToken(client, publicKey, dataDir); err != nil {
			return err
		}
	} else {
		log.LogWarn("PUBLIC_KEY not provided, running without authentication")
	}

	var apiBot *tgbotapi.BotAPI
	apiBotToken := os.Getenv("API_BOT_TOKEN")
	apiBotChatID := os.Getenv("API_BOT_CHAT_ID")

	if apiBotToken != "" {
		var err error
		apiBot, err = tgbotapi.NewBotAPI(apiBotToken)
		if err != nil {
			log.LogWarn("Failed to initialize API bot (continuing without it)", zap.Error(err))
		} else {
			log.LogSuccess("API Bot authorized", zap.String("username", apiBot.Self.UserName))
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	minBTCAmount := 0.0025

	wg.Add(1)
	go func() {
		defer wg.Done()
		bots_monitor.RunBigSalesBuysMonitor(apiBot, client, apiBotChatID, minBTCAmount, nil, "", nil, 0)
	}()

	log.LogSuccess("Big Sales monitor is running", zap.String("status", "active"))

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
		log.LogSuccess("Big Sales monitor stopped gracefully")
	case <-time.After(10 * time.Second):
		log.LogWarn("Timeout waiting for monitor to stop")
	}

	return nil
}

func ensureValidToken(client *flashnet.Client, publicKey string, dataDir string) error {
	tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			client.SetJWT(tokenFile.AccessToken)
			log.LogInfo("Using saved JWT token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			return nil
		} else {
			log.LogWarn("Saved token is expired or invalid, refreshing...")
		}
	}

	log.LogInfo("Getting new challenge...")
	ctx := context.Background()
	_, err = client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		log.LogError("Failed to get challenge", zap.Error(err))
		return fmt.Errorf("failed to get challenge: %w", err)
	}

	log.LogInfo("Signing challenge automatically...")
	signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
	cmd := exec.Command("node", signChallengePath)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.LogError("Failed to sign challenge", zap.Error(err), zap.String("output", string(output)))
		return fmt.Errorf("failed to sign challenge: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
	if err != nil || sigFile.Signature == "" {
		return fmt.Errorf("signature file not found after signing")
	}

	log.LogInfo("Verifying signature...")
	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		log.LogError("Failed to verify signature", zap.Error(err))
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	log.LogSuccess("Token refreshed successfully")
	return nil
}
