package main

// for challenge Flashnet API
// go run cmd/auth/get_challenge/main.go

import (
	"context"
	"os"
	"spark-wallet/http_client/system_works"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load
	godotenv.Load(".env")

	publicKey := os.Getenv("PUBLIC_KEY")
	if publicKey == "" {
		system_works.LogError("PUBLIC_KEY not found in .env file")
		system_works.Logger.Fatal("PUBLIC_KEY not found in .env file")
	}

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	system_works.LogInfo("Getting challenge from API...")
	system_works.LogInfo("Public Key", zap.String("publicKey", publicKey))
	system_works.LogInfo("Network", zap.String("network", network))

	// Create API
	client := system_works.NewAMMClient(network)

	// Get challenge and save in file
	dataDir := "data_in"
	ctx := context.Background()
	_, err := client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		system_works.LogError("Failed to get challenge", zap.Error(err))
		system_works.Logger.Fatal("Failed to get challenge", zap.Error(err))
	}

	system_works.LogInfo("Challenge saved to data_in/challenge.json")
	system_works.LogInfo("Next step: sign the challenge using 'make sign'")
}
