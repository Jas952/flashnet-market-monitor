package main

// for and JWT token
// go run cmd/auth/verify_token/main.go

import (
	"context"
	"os"
	"spark-wallet/http_client/system_works"
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

	tokenFile, err := system_works.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := system_works.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			system_works.LogSuccess("Valid token already exists",
				zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			system_works.LogInfo("Token is still valid, no need to verify")
			return
		} else {
			system_works.LogWarn("Existing token is expired, verifying signature to get new one...")
		}
	}

	system_works.LogInfo("Verifying signature...")

	sigFile, err := system_works.LoadSignatureFromFile(dataDir)
	if err != nil {
		system_works.LogError("Failed to load signature file", zap.Error(err))
		system_works.LogInfo("Make sure signature.json exists and contains signature field")
		system_works.Logger.Fatal("Failed to load signature file", zap.Error(err))
	}

	if sigFile.PublicKey == "" {
		system_works.LogError("Public key is empty - challenge.json may be missing")
		system_works.Logger.Fatal("Public key is empty")
	}

	if sigFile.Signature == "" {
		system_works.LogError("Signature is empty in signature.json file")
		system_works.LogInfo("Please sign the challengeString from challenge.json and save signature to signature.json")
		system_works.Logger.Fatal("Signature is empty in signature.json file")
	}

	system_works.LogInfo("Signature loaded from file", zap.String("signature", sigFile.Signature[:20]))
	system_works.LogInfo("Using public key", zap.String("publicKey", sigFile.PublicKey[:20]))

	// Create API
	client := system_works.NewAMMClient(network)

	// on and save token in file
	system_works.LogInfo("Verifying signature with API...")
	ctx := context.Background()
	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		// Check, "already signed in"
		if system_works.IsAlreadySignedInError(err) {
			system_works.LogWarn("User already has active session")
			system_works.LogInfo("Checking if existing token is valid...")

			tokenFile, err := system_works.LoadTokenFromFile(dataDir)
			if err == nil && tokenFile.AccessToken != "" {
				expiresAt, err := system_works.GetTokenExpirationTime(tokenFile.AccessToken)
				if err == nil && expiresAt > time.Now().Unix() {
					system_works.LogSuccess("Using existing valid token",
						zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
					return
				}
			}

			system_works.LogWarn("Active session exists but token file is missing or expired")
			system_works.LogInfo("Please wait for current session to expire (1 hour) or use existing token if available")
			return
		}

		system_works.LogError("Failed to verify signature", zap.Error(err))
		system_works.Logger.Fatal("Failed to verify signature", zap.Error(err))
	}

	tokenFileData, err := system_works.LoadTokenFromFile(dataDir)
	if err == nil && tokenFileData.ExpiresAt > 0 {
		system_works.LogInfo("Token expires at", zap.String("expiresAt", time.Unix(tokenFileData.ExpiresAt, 0).Format(time.RFC3339)))
	}
}
