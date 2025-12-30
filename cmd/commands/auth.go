package commands

// Commands for Flashnet API authentication
// Supports challenge retrieval, signing, and verification
// Implements full authentication cycle (challenge + sign + verify)
// Saves challenge, signature, and token to files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"spark-wallet/internal/clients_api/flashnet"
	executil "spark-wallet/internal/infra/exec"
	storage "spark-wallet/internal/infra/fs"
	"spark-wallet/internal/infra/log"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands for Flashnet API",
	Long:  `Manage authentication with Flashnet API. Includes subcommands for challenge, verify, and full auth flow.`,
}

var authChallengeCmd = &cobra.Command{
	Use:   "challenge",
	Short: "Get challenge from Flashnet API",
	Long:  `Request a challenge from Flashnet API and save it to data_in/challenge.json`,
	RunE:  runAuthChallenge,
}

var authVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify signature and get JWT token",
	Long:  `Verify the signed challenge and obtain JWT token from Flashnet API`,
	RunE:  runAuthVerify,
}

var authFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Complete authentication flow (challenge + sign + verify)",
	Long:  `Perform complete authentication: get challenge, sign it, and verify to obtain JWT token`,
	RunE:  runAuthFull,
}

func init() {
	authCmd.AddCommand(authChallengeCmd)
	authCmd.AddCommand(authVerifyCmd)
	authCmd.AddCommand(authFullCmd)
}

func runAuthChallenge(cmd *cobra.Command, args []string) error {
	godotenv.Load(".env")

	publicKey := os.Getenv("PUBLIC_KEY")
	if publicKey == "" {
		log.LogError("PUBLIC_KEY not found in .env file")
		return fmt.Errorf("PUBLIC_KEY not found in .env file")
	}

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	log.LogInfo("Getting challenge from API...")
	log.LogInfo("Public Key", zap.String("publicKey", publicKey))
	log.LogInfo("Network", zap.String("network", network))

	client := flashnet.NewAMMClient(network)
	dataDir := "data_in"
	ctx := context.Background()

	_, err := client.GetChallengeAndSave(ctx, dataDir, publicKey)
	if err != nil {
		log.LogError("Failed to get challenge", zap.Error(err))
		return fmt.Errorf("failed to get challenge: %w", err)
	}

	log.LogInfo("Challenge saved to data_in/challenge.json")
	log.LogInfo("Next step: sign the challenge using 'make sign' or 'flashnet-api auth verify'")
	return nil
}

func runAuthVerify(cmd *cobra.Command, args []string) error {
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
			return nil
		} else {
			log.LogWarn("Existing token is expired, verifying signature to get new one...")
		}
	}

	log.LogInfo("Verifying signature...")

	sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
	if err != nil {
		log.LogError("Failed to load signature file", zap.Error(err))
		log.LogInfo("Make sure signature.json exists and contains signature field")
		return fmt.Errorf("failed to load signature file: %w", err)
	}

	if sigFile.PublicKey == "" {
		log.LogError("Public key is empty - challenge.json may be missing")
		return fmt.Errorf("public key is empty")
	}

	if sigFile.Signature == "" {
		log.LogError("Signature is empty in signature.json file")
		log.LogInfo("Please sign the challengeString from challenge.json and save signature to signature.json")
		return fmt.Errorf("signature is empty in signature.json file")
	}

	log.LogInfo("Signature loaded from file", zap.String("signature", sigFile.Signature[:20]))
	log.LogInfo("Using public key", zap.String("publicKey", sigFile.PublicKey[:20]))

	client := flashnet.NewAMMClient(network)

	log.LogInfo("Verifying signature with API...")
	ctx := context.Background()
	_, err = client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
	if err != nil {
		if flashnet.IsAlreadySignedInError(err) {
			log.LogWarn("User already has active session")
			log.LogInfo("Checking if existing token is valid...")

			tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
			if err == nil && tokenFile.AccessToken != "" {
				expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
				if err == nil && expiresAt > time.Now().Unix() {
					log.LogSuccess("Using existing valid token",
						zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
					return nil
				}
			}

			log.LogWarn("Active session exists but token file is missing or expired")
			log.LogInfo("Please wait for current session to expire (1 hour) or use existing token if available")
			return nil
		}

		log.LogError("Failed to verify signature", zap.Error(err))
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	tokenFileData, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFileData.ExpiresAt > 0 {
		log.LogInfo("Token expires at", zap.String("expiresAt", time.Unix(tokenFileData.ExpiresAt, 0).Format(time.RFC3339)))
	}

	return nil
}

func runAuthFull(cmd *cobra.Command, args []string) error {
	if err := runAuthChallenge(cmd, args); err != nil {
		return fmt.Errorf("failed to get challenge: %w", err)
	}

	log.LogInfo("Signing challenge...")
	signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
	output, err := executil.RunNodeScript(signChallengePath, 30*time.Second)
	if err != nil {
		log.LogError("Failed to sign challenge", zap.Error(err), zap.String("output", string(output)))
		return fmt.Errorf("failed to sign challenge: %w", err)
	}

	log.LogSuccess("Challenge signed successfully")

	// Wait for signature file to be written
	dataDir := "data_in"
	signatureFilePath := filepath.Join(dataDir, "signature.json")
	if err := storage.WaitForFile(signatureFilePath, 3*time.Second); err != nil {
		log.LogError("Signature file not created within timeout", zap.Error(err))
		return fmt.Errorf("signature file not created: %w", err)
	}

	// Step 3: Verify signature
	if err := runAuthVerify(cmd, args); err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	log.LogSuccess("Authentication completed successfully")
	return nil
}
