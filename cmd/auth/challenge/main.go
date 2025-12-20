package main

// for challenge Flashnet API
// go run cmd/auth/get_challenge/main.go

import (
	"context"
	"os"

	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/infra/log"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load
	godotenv.Load(".env")

	publicKey := os.Getenv("PUBLIC_KEY")
	if publicKey == "" {
		log.LogError("PUBLIC_KEY not found in .env file")
		log.Logger.Fatal("PUBLIC_KEY not found in .env file")
	}

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	log.LogInfo("Getting challenge from API...")
	log.LogInfo("Public Key", zap.String("publicKey", publicKey))
	log.LogInfo("Network", zap.String("network", network))

	// Create API
	client := flashnet.NewAMMClient(network)

	// Get challenge and save in file
	dataDir := "data_in"
	ctx := context.Background()
	_, err := client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		log.LogError("Failed to get challenge", zap.Error(err))
		log.Logger.Fatal("Failed to get challenge", zap.Error(err))
	}

	log.LogInfo("Challenge saved to data_in/challenge.json")
	log.LogInfo("Next step: sign the challenge using 'make sign'")
}
