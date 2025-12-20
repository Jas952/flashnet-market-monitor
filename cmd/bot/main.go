package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"spark-wallet/http_client/bot"
	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/infra/config"
	storage "spark-wallet/internal/infra/fs"
	logging "spark-wallet/internal/infra/log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// main starts Telegram bot and monitors (Flashnet + reports).
func main() {
	// Load configuration from env, flags and files via unified interface
	cfg, err := config.LoadConfig()
	if err != nil {
		logging.LogError("Failed to load config", zap.Error(err))
		logging.Logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Verify that public key matches the correct key from spark-cli
	// Correct public key from Spark SDK: 038ad2deab88fa2f278ad895f61254a804370d987db61301a7d6872df4231b6597
	const expectedPublicKey = "038ad2deab88fa2f278ad895f61254a804370d987db61301a7d6872df4231b6597"
	if cfg.Flashnet.PublicKey != "" && cfg.Flashnet.PublicKey != expectedPublicKey {
		logging.LogWarn("Public key does not match expected Spark SDK public key",
			zap.String("provided", cfg.Flashnet.PublicKey),
			zap.String("expected", expectedPublicKey))
		logging.LogInfo("Make sure PUBLIC_KEY matches the key from spark-cli (Spark SDK)")
	}

	// Use dataDir from config or default value
	dataDir := cfg.App.DataDir
	if dataDir == "" {
		dataDir = "data_in"
	}

	// Flashnet AMM API client.
	client := flashnet.NewAMMClient(cfg.Flashnet.Network)

	// Authentication via challenge/signature files (if PUBLIC_KEY is provided).
	if cfg.Flashnet.PublicKey != "" {
		// Check if there's already a saved token
		tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
		if err == nil && tokenFile.AccessToken != "" {
			// Check token expiration time (extract expiration from token)
			expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
			if err == nil && expiresAt > time.Now().Unix() {
				// Token is valid, use it
				client.SetJWT(tokenFile.AccessToken)
				logging.LogInfo("Using saved JWT token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			} else {
				logging.LogWarn("Saved token is expired or invalid")
				logging.LogInfo("Need to get new challenge and verify signature")
			}
		}

		// If token is missing or expired, get challenge (automatically saved to file)
		if client.GetJWT() == "" {
			logging.LogInfo("Getting challenge...")
			ctx := context.Background()
			_, err := client.GetChallengeAndSave(ctx, dataDir, cfg.Flashnet.PublicKey)
			if err != nil {
				logging.LogError("Failed to get challenge", zap.Error(err))
				logging.Logger.Fatal("Failed to get challenge", zap.Error(err))
			}
			// GetChallengeAndSave already logs success with execution time
			logging.LogInfo("Challenge saved, loading challenge file...")

			// Load current challenge to verify requestId
			challengeFile, err := flashnet.LoadChallengeFromFile(dataDir)
			if err != nil {
				logging.LogError("Failed to load challenge file", zap.Error(err))
				logging.Logger.Fatal("Failed to load challenge file", zap.Error(err))
			}
			logging.LogInfo("Challenge file loaded", zap.String("requestId", challengeFile.RequestID))

			// Check if signature file exists
			logging.LogInfo("Checking for signature file...")
			sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
			if err != nil {
				logging.LogInfo("No signature file found or failed to load", zap.Error(err))
				sigFile = nil // Ensure sigFile = nil on error
			} else if sigFile == nil {
				logging.LogInfo("Signature file is nil")
			} else {
				logging.LogInfo("Loaded signature file", zap.String("requestId", sigFile.RequestID), zap.Bool("hasSignature", sigFile.Signature != ""), zap.String("challengeRequestId", challengeFile.RequestID))
			}

			// Verify signature if it exists
			if sigFile != nil && sigFile.Signature != "" {
				// Check if signature matches current challenge (by requestId)
				if sigFile.RequestID != challengeFile.RequestID {
					logging.LogWarn("Signature requestId does not match current challenge requestId, will create new signature",
						zap.String("signatureRequestId", sigFile.RequestID),
						zap.String("challengeRequestId", challengeFile.RequestID))
					// Remove old signature so bot creates a new one
					sigFile.Signature = ""
				} else {
					logging.LogInfo("Found signature file matching current challenge, verifying...")
					// Send signature to server and get JWT token (automatically saved to file)
					// Use publicKey from sigFile as it's already synced from challenge.json
					ctx := context.Background()
					_, err := client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
					if err != nil {
						// If error is "already signed in", it's normal - token is already valid
						if flashnet.IsAlreadySignedInError(err) {
							logging.LogInfo("User already signed in, token is valid")
							// Try to load existing token
							tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
							if err == nil && tokenFile.AccessToken != "" {
								expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
								if err == nil && expiresAt > time.Now().Unix() {
									client.SetJWT(tokenFile.AccessToken)
									logging.LogInfo("Using existing valid token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
								} else {
									// Token expired, need to create new signature
									logging.LogWarn("Existing token expired, will create new signature")
									sigFile.Signature = ""
								}
							} else {
								// No token found, need to create new signature
								logging.LogWarn("No valid token found, will create new signature")
								sigFile.Signature = ""
							}
						} else {
							logging.LogError("Failed to verify signature", zap.Error(err))
							// If signature verification failed, it might be for old challenge
							// Remove signature and create new one
							logging.LogWarn("Signature verification failed, will create new signature")
							sigFile.Signature = ""
						}
					} else {
						// VerifySignatureAndSave already logs success with execution time
						// Signature successfully verified, token obtained
					}
				}
			}

			// If signature is missing or was removed, create new one
			if sigFile == nil || sigFile.Signature == "" {
				// No signature - automatically call sign-challenge.mjs
				logging.LogInfo("Signature not found, signing challenge automatically...")
				signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
				logging.LogInfo("Executing sign script", zap.String("path", signChallengePath))

				// Execute Node.js script to sign challenge
				startTime := time.Now()
				cmd := exec.Command("node", signChallengePath)
				cmd.Dir = "." // Work from project root
				logging.LogInfo("Running sign-challenge.mjs...")
				output, err := cmd.CombinedOutput()
				duration := time.Since(startTime).Milliseconds()
				if err != nil {
					logging.LogError("Failed to sign challenge",
						zap.Error(err),
						zap.String("output", string(output)),
						zap.Int64("duration_ms", duration))
					logging.LogWarn("Bot will run without authentication. Please sign manually:")
					logging.LogInfo("1. Run: make sign")
					logging.LogInfo("2. Restart the bot")
				} else {
					logging.LogSuccess("Challenge signed successfully", zap.Int64("duration_ms", duration))

					// Check that signature appeared in file
					time.Sleep(500 * time.Millisecond) // Give time for file write
					sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
					if err == nil && sigFile.Signature != "" {
						logging.LogInfo("Verifying signature...")
						ctx := context.Background()
						_, err := client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
						if err != nil {
							logging.LogError("Failed to verify signature", zap.Error(err))
							logging.Logger.Fatal("Failed to verify signature", zap.Error(err))
						}
						// VerifySignatureAndSave
					} else {
						logging.LogWarn("Signature file not found after signing. Bot will run without authentication.")
					}
				}
			}
		}
	} else {
		// If keys not provided, work without authentication
		logging.LogWarn("PUBLIC_KEY not provided, running without authentication")
	}

	// Initialize API bot for swap notifications (priority)
	var apiBot *tgbotapi.BotAPI
	if cfg.Telegram.ApiBotToken != "" {
		apiBot, err = tgbotapi.NewBotAPI(cfg.Telegram.ApiBotToken)
		if err != nil {
			logging.LogWarn("Failed to initialize API bot (continuing without it)", zap.Error(err))
		} else {
			logging.LogSuccess("API Bot authorized", zap.String("username", apiBot.Self.UserName))

		}
	}

	// Initialize first Telegram bot (if ApiBotToken not provided)
	var bot1 *tgbotapi.BotAPI
	if cfg.Telegram.Bot1Token != "" {
		bot1, err = tgbotapi.NewBotAPI(cfg.Telegram.Bot1Token)
		if err != nil {
			logging.LogError("Failed to initialize bot 1", zap.Error(err))
			logging.Logger.Fatal("Failed to initialize bot 1", zap.Error(err))
		}
		logging.LogSuccess("Bot 1 authorized", zap.String("username", bot1.Self.UserName))
	} else if apiBot == nil {
		// If neither Bot1Token nor ApiBotToken provided - it's an error
		logging.Logger.Fatal("No bot token provided: either TELEGRAM_BOT1_TOKEN or API_BOT_TOKEN is required")
	}

	// Initialize second Telegram bot (optional, for future use)
	var bot2 *tgbotapi.BotAPI
	if cfg.Telegram.Bot2Token != "" {
		bot2, err = tgbotapi.NewBotAPI(cfg.Telegram.Bot2Token)
		if err != nil {
			// If failed to initialize second bot, just show warning
			// and continue without it
			logging.LogWarn("Failed to initialize bot 2 (continuing without it)", zap.Error(err))
		} else {
			logging.LogSuccess("Bot 2 authorized", zap.String("username", bot2.Self.UserName))
		}
	}

	// Start big sales monitoring in separate goroutine
	// Use API bot if configured, otherwise use bot1
	bigSalesBot := apiBot
	bigSalesChatID := cfg.Telegram.ApiBotChatID
	if bigSalesBot == nil || bigSalesChatID == "" {
		bigSalesBot = bot1
		bigSalesChatID = cfg.Telegram.BigSalesChatID
	}
	// Get minimum amount for main chat (default 0.0025)
	bigSalesMinBTCAmount := cfg.Telegram.BigSalesMinBTCAmount
	if bigSalesMinBTCAmount == 0 {
		bigSalesMinBTCAmount = 0.0025
	}

	// Configure filtered chat (if configured)
	var filteredBot *tgbotapi.BotAPI
	var filteredChatID string
	var filteredTokensList []string
	var filteredMinBTCAmount float64
	if cfg.Telegram.FilteredChatID != "" {
		filteredBot = apiBot
		if filteredBot == nil {
			filteredBot = bot1
		}
		filteredChatID = cfg.Telegram.FilteredChatID

		// Load filtered tokens from file (priority) or from config (fallback)
		var err error
		filteredTokensList, err = storage.LoadFilteredTokens()
		if err != nil {
			logging.LogWarn("Failed to load filtered tokens from file, using config", zap.Error(err))
			// Fallback to config (from config.yaml or .env)
			if len(cfg.Telegram.FilteredTokens) > 0 {
				filteredTokensList = cfg.Telegram.FilteredTokens
				// Save tokens from config to file for future use
				if saveErr := storage.SaveFilteredTokens(filteredTokensList); saveErr != nil {
					logging.LogWarn("Failed to save filtered tokens from config to file", zap.Error(saveErr))
				} else {
					logging.LogInfo("Migrated filtered tokens from config to file", zap.Int("count", len(filteredTokensList)))
				}
			} else {
				filteredTokensList = []string{}
			}
		} else if len(filteredTokensList) == 0 && len(cfg.Telegram.FilteredTokens) > 0 {
			// If file is empty but config has tokens - save them to file
			if saveErr := storage.SaveFilteredTokens(cfg.Telegram.FilteredTokens); saveErr != nil {
				logging.LogWarn("Failed to save filtered tokens from config to file", zap.Error(saveErr))
			} else {
				filteredTokensList = cfg.Telegram.FilteredTokens
				logging.LogInfo("Migrated filtered tokens from config to file", zap.Int("count", len(filteredTokensList)))
			}
		}

		// Get minimum amount for filtered chat (default 0.01)
		filteredMinBTCAmount = cfg.Telegram.FilteredMinBTCAmount
		if filteredMinBTCAmount == 0 {
			filteredMinBTCAmount = 0.01
		}
		logging.LogInfo("Filtered tokens monitor configured",
			zap.String("chatID", filteredChatID),
			zap.Int("tokensCount", len(filteredTokensList)),
			zap.Float64("minBTCAmount", filteredMinBTCAmount))

		// Start command handler for filtered chat
		if filteredBot != nil {
			go bot.RunCommandHandler(filteredBot, filteredChatID, client)
			// Check and send stats on startup if not sent today
			// Start stats monitor in separate goroutine
			statsSendTime := cfg.Telegram.StatsSendTime
			if statsSendTime == "" {
				statsSendTime = "10:00" // Default value
			}
			go bot.RunStatsMonitor(filteredBot, filteredChatID, statsSendTime)
		}
	}

	// Start hot token monitor in separate goroutine (requires FILTERED_CHAT_ID)
	// Hot token monitor checks ALL tokens from recent swaps and sends notifications to FILTERED_CHAT_ID
	if cfg.Telegram.FilteredChatID != "" {
		hotTokenBot := apiBot
		if hotTokenBot == nil {
			hotTokenBot = bot1
		}
		if hotTokenBot == nil {
			hotTokenBot = bot2
		}

		hotTokenSwapsCount := cfg.Telegram.HotTokenSwapsCount
		if hotTokenSwapsCount == 0 {
			hotTokenSwapsCount = 6 // Default value
		}
		hotTokenMinAddresses := cfg.Telegram.HotTokenMinAddresses
		if hotTokenMinAddresses == 0 {
			hotTokenMinAddresses = 3 // Default value
		}
		checkInterval := cfg.App.CheckInterval
		if checkInterval == 0 {
			checkInterval = 30 // Default value
		}

		if hotTokenBot != nil {
			logging.LogInfo("Hot token monitor configured",
				zap.Int("swapsCount", hotTokenSwapsCount),
				zap.Int("minAddresses", hotTokenMinAddresses),
				zap.Int("checkInterval", checkInterval),
				zap.String("chatID", cfg.Telegram.FilteredChatID))
			go bot.RunHotTokenMonitor(hotTokenBot, client, cfg.Telegram.FilteredChatID, hotTokenSwapsCount, hotTokenMinAddresses, checkInterval)
		} else {
			logging.LogWarn("Hot token monitor not started: no bot available",
				zap.String("chatID", cfg.Telegram.FilteredChatID))
		}
	} else {
		logging.LogDebug("Hot token monitor skipped: FILTERED_CHAT_ID not configured")
	}

	// Start unified monitoring that sends to both chats simultaneously
	go bot.RunBigSalesBuysMonitor(bigSalesBot, client, bigSalesChatID, bigSalesMinBTCAmount, filteredBot, filteredChatID, filteredTokensList, filteredMinBTCAmount)

	// Start holders dynamics monitoring
	go bot.RunHoldersDynamicMonitor()

	// Keep main goroutine running
	// select {} is an infinite wait loop that blocks execution
	// Program will run until we press Ctrl+C
	logging.LogSuccess("Bots are running", zap.String("status", "active"))
	select {}
}
