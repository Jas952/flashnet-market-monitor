package main

// Big Sales Monitor Telegram
// go run cmd/big_sales/main.go

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"spark-wallet/http_client/bot"
	"spark-wallet/http_client/system_works"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load
	godotenv.Load(".env")

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	publicKey := os.Getenv("PUBLIC_KEY")
	dataDir := "data_in"

	system_works.LogInfo("Starting Big Sales Monitor (without Telegram)...")
	system_works.LogInfo("Network", zap.String("network", network))

	// Create API
	client := system_works.NewAMMClient(network)

	// Check and update token
	if publicKey != "" {
		ensureValidToken(client, publicKey, dataDir)
	} else {
		system_works.LogWarn("PUBLIC_KEY not provided, running without authentication")
	}

	// Initialize API-, if token
	var apiBot *tgbotapi.BotAPI
	apiBotToken := os.Getenv("API_BOT_TOKEN")
	apiBotChatID := os.Getenv("API_BOT_CHAT_ID")

	if apiBotToken != "" {
		var err error
		apiBot, err = tgbotapi.NewBotAPI(apiBotToken)
		if err != nil {
			system_works.LogWarn("Failed to initialize API bot (continuing without it)", zap.Error(err))
		} else {
			system_works.LogSuccess("API Bot authorized", zap.String("username", apiBot.Self.UserName))
		}
	}

	// Start API-, if
	// Use amount by default 0.0025 BTC
	// for Telegram nil for
	minBTCAmount := 0.0025
	bot.RunBigSalesBuysMonitor(apiBot, client, apiBotChatID, minBTCAmount, nil, "", nil, 0)
}

func ensureValidToken(client *system_works.Client, publicKey string, dataDir string) {
	// Check, token
	tokenFile, err := system_works.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := system_works.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			// token use
			client.SetJWT(tokenFile.AccessToken)
			system_works.LogInfo("Using saved JWT token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			return
		} else {
			system_works.LogWarn("Saved token is expired or invalid, refreshing...")
		}
	}

	// token or - get
	system_works.LogInfo("Getting new challenge...")
	ctx := context.Background()
	_, err = client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		system_works.LogError("Failed to get challenge", zap.Error(err))
		system_works.Logger.Fatal("Failed to get challenge", zap.Error(err))
	}

	system_works.LogInfo("Signing challenge automatically...")
	signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
	cmd := exec.Command("node", signChallengePath)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		system_works.LogError("Failed to sign challenge",
			zap.Error(err),
			zap.String("output", string(output)))
		system_works.Logger.Fatal("Failed to sign challenge. Please run: make sign")
	}

	// time on file
	time.Sleep(500 * time.Millisecond)

	sigFile, err := system_works.LoadSignatureFromFile(dataDir)
	if err != nil || sigFile.Signature == "" {
		system_works.Logger.Fatal("Signature file not found after signing")
	}

	system_works.LogInfo("Verifying signature...")
	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		system_works.LogError("Failed to verify signature", zap.Error(err))
		system_works.Logger.Fatal("Failed to verify signature", zap.Error(err))
	}

	system_works.LogSuccess("Token refreshed successfully")
}
