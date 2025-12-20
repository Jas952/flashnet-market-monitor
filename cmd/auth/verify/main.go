package main

// for and JWT token
// go run cmd/auth/verify_token/main.go

import (
	"context"
	"os"

	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/infra/log"
	"time"

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

	dataDir := "data_in"

	tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			log.LogSuccess("Valid token already exists",
				zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			log.LogInfo("Token is still valid, no need to verify")
			return
		} else {
			log.LogWarn("Existing token is expired, verifying signature to get new one...")
		}
	}

	log.LogInfo("Verifying signature...")

	sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
	if err != nil {
		log.LogError("Failed to load signature file", zap.Error(err))
		log.LogInfo("Make sure signature.json exists and contains signature field")
		log.Logger.Fatal("Failed to load signature file", zap.Error(err))
	}

	if sigFile.PublicKey == "" {
		log.LogError("Public key is empty - challenge.json may be missing")
		log.Logger.Fatal("Public key is empty")
	}

	if sigFile.Signature == "" {
		log.LogError("Signature is empty in signature.json file")
		log.LogInfo("Please sign the challengeString from challenge.json and save signature to signature.json")
		log.Logger.Fatal("Signature is empty in signature.json file")
	}

	log.LogInfo("Signature loaded from file", zap.String("signature", sigFile.Signature[:20]))
	log.LogInfo("Using public key", zap.String("publicKey", sigFile.PublicKey[:20]))

	// Create API
	client := flashnet.NewAMMClient(network)

	// on and save token in file
	log.LogInfo("Verifying signature with API...")
	ctx := context.Background()
	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		// Check, "already signed in"
		if flashnet.IsAlreadySignedInError(err) {
			log.LogWarn("User already has active session")
			log.LogInfo("Checking if existing token is valid...")

			tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
			if err == nil && tokenFile.AccessToken != "" {
				expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
				if err == nil && expiresAt > time.Now().Unix() {
					log.LogSuccess("Using existing valid token",
						zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
					return
				}
			}

			log.LogWarn("Active session exists but token file is missing or expired")
			log.LogInfo("Please wait for current session to expire (1 hour) or use existing token if available")
			return
		}

		log.LogError("Failed to verify signature", zap.Error(err))
		log.Logger.Fatal("Failed to verify signature", zap.Error(err))
	}

	tokenFileData, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFileData.ExpiresAt > 0 {
		log.LogInfo("Token expires at", zap.String("expiresAt", time.Unix(tokenFileData.ExpiresAt, 0).Format(time.RFC3339)))
	}
}
