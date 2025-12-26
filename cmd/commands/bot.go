package commands

// Command to run the full bot with all monitors
// Initializes configuration and Flashnet API authentication
// Starts all monitors (Big Sales, Holders, Hot Token, Stats, BTC Spark)
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
	"spark-wallet/internal/infra/config"
	storage "spark-wallet/internal/infra/fs"
	logging "spark-wallet/internal/infra/log"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Run full bot with all monitors (auth + Telegram)",
	Long:  `Run the complete bot with all monitoring features including Big Sales, Hot Token, Holders, and Statistics monitors.`,
	RunE:  runBot,
}

func runBot(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		logging.LogError("Failed to load config", zap.Error(err))
		return fmt.Errorf("failed to load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup

	const expectedPublicKey = "038ad2deab88fa2f278ad895f61254a804370d987db61301a7d6872df4231b6597"
	if cfg.Flashnet.PublicKey != "" && cfg.Flashnet.PublicKey != expectedPublicKey {
		logging.LogWarn("Public key does not match expected Spark SDK public key",
			zap.String("provided", cfg.Flashnet.PublicKey),
			zap.String("expected", expectedPublicKey))
		logging.LogInfo("Make sure PUBLIC_KEY matches the key from spark-cli (Spark SDK)")
	}

	dataDir := cfg.App.DataDir
	if dataDir == "" {
		dataDir = "data_in"
	}

	client := flashnet.NewAMMClient(cfg.Flashnet.Network)

	if cfg.Flashnet.PublicKey != "" {
		if err := handleAuthentication(ctx, client, cfg, dataDir); err != nil {
			return err
		}
	} else {
		logging.LogWarn("PUBLIC_KEY not provided, running without authentication")
	}

	apiBot, bot1, bot2, err := initializeBots(cfg)
	if err != nil {
		return err
	}

	if err := startMonitors(ctx, &wg, cfg, client, apiBot, bot1, bot2); err != nil {
		return err
	}

	logging.LogSuccess("Bots are running", zap.String("status", "active"))

	<-ctx.Done()
	logging.LogInfo("Shutdown signal received, gracefully stopping all monitors...")

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logging.LogSuccess("All monitors stopped gracefully")
	case <-time.After(10 * time.Second):
		logging.LogWarn("Timeout waiting for monitors to stop, forcing shutdown")
	}

	return nil
}

func handleAuthentication(ctx context.Context, client *flashnet.Client, cfg *config.Config, dataDir string) error {
	tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tokenFile.AccessToken != "" {
		expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
		if err == nil && expiresAt > time.Now().Unix() {
			client.SetJWT(tokenFile.AccessToken)
			logging.LogInfo("Using saved JWT token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
			return nil
		} else {
			logging.LogWarn("Saved token is expired or invalid")
			logging.LogInfo("Need to get new challenge and verify signature")
		}
	}

	if client.GetJWT() == "" {
		logging.LogInfo("Getting challenge...")
		_, err := client.GetChallengeAndSave(ctx, dataDir, cfg.Flashnet.PublicKey)
		if err != nil {
			logging.LogError("Failed to get challenge", zap.Error(err))
			return fmt.Errorf("failed to get challenge: %w", err)
		}

		challengeFile, err := flashnet.LoadChallengeFromFile(dataDir)
		if err != nil {
			logging.LogError("Failed to load challenge file", zap.Error(err))
			return fmt.Errorf("failed to load challenge file: %w", err)
		}
		logging.LogInfo("Challenge file loaded", zap.String("requestId", challengeFile.RequestID))

		sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
		if err != nil {
			logging.LogInfo("No signature file found or failed to load", zap.Error(err))
			sigFile = nil
		}

		if sigFile != nil && sigFile.Signature != "" {
			if sigFile.RequestID != challengeFile.RequestID {
				logging.LogWarn("Signature requestId does not match current challenge requestId, will create new signature")
				sigFile.Signature = ""
			} else {
				logging.LogInfo("Found signature file matching current challenge, verifying...")
				_, err := client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
				if err != nil {
					if flashnet.IsAlreadySignedInError(err) {
						logging.LogInfo("User already signed in, token is valid")
						tokenFile, err := flashnet.LoadTokenFromFile(dataDir)
						if err == nil && tokenFile.AccessToken != "" {
							expiresAt, err := flashnet.GetTokenExpirationTime(tokenFile.AccessToken)
							if err == nil && expiresAt > time.Now().Unix() {
								client.SetJWT(tokenFile.AccessToken)
								logging.LogInfo("Using existing valid token", zap.String("expiresAt", time.Unix(expiresAt, 0).Format(time.RFC3339)))
								return nil
							}
						}
					} else {
						logging.LogError("Failed to verify signature", zap.Error(err))
						logging.LogWarn("Signature verification failed, will create new signature")
						sigFile.Signature = ""
					}
				}
			}
		}

		if sigFile == nil || sigFile.Signature == "" {
			logging.LogInfo("Signature not found, signing challenge automatically...")
			signChallengePath := filepath.Join("spark-cli", "sign-challenge.mjs")
			cmd := exec.Command("node", signChallengePath)
			cmd.Dir = "."
			output, err := cmd.CombinedOutput()
			if err != nil {
				logging.LogError("Failed to sign challenge", zap.Error(err), zap.String("output", string(output)))
				logging.LogWarn("Bot will run without authentication. Please sign manually:")
				logging.LogInfo("1. Run: make sign")
				logging.LogInfo("2. Restart the bot")
				return nil
			}

			logging.LogSuccess("Challenge signed successfully")
			time.Sleep(500 * time.Millisecond)
			sigFile, err := flashnet.LoadSignatureFromFile(dataDir)
			if err == nil && sigFile.Signature != "" {
				logging.LogInfo("Verifying signature...")
				_, err := client.VerifySignatureAndSave(ctx, dataDir, sigFile.PublicKey, sigFile.Signature)
				if err != nil {
					logging.LogError("Failed to verify signature", zap.Error(err))
					return fmt.Errorf("failed to verify signature: %w", err)
				}
			} else {
				logging.LogWarn("Signature file not found after signing. Bot will run without authentication.")
			}
		}
	}
	return nil
}

func initializeBots(cfg *config.Config) (*tgbotapi.BotAPI, *tgbotapi.BotAPI, *tgbotapi.BotAPI, error) {
	var apiBot *tgbotapi.BotAPI
	if cfg.Telegram.ApiBotToken != "" {
		var err error
		apiBot, err = tgbotapi.NewBotAPI(cfg.Telegram.ApiBotToken)
		if err != nil {
			logging.LogWarn("Failed to initialize API bot (continuing without it)", zap.Error(err))
		} else {
			logging.LogSuccess("API Bot authorized", zap.String("username", apiBot.Self.UserName))
		}
	}

	var bot1 *tgbotapi.BotAPI
	if cfg.Telegram.Bot1Token != "" {
		var err error
		bot1, err = tgbotapi.NewBotAPI(cfg.Telegram.Bot1Token)
		if err != nil {
			logging.LogError("Failed to initialize bot 1", zap.Error(err))
			return nil, nil, nil, fmt.Errorf("failed to initialize bot 1: %w", err)
		}
		logging.LogSuccess("Bot 1 authorized", zap.String("username", bot1.Self.UserName))
	} else if apiBot == nil {
		return nil, nil, nil, fmt.Errorf("no bot token provided: either TELEGRAM_BOT1_TOKEN or API_BOT_TOKEN is required")
	}

	var bot2 *tgbotapi.BotAPI
	if cfg.Telegram.Bot2Token != "" {
		var err error
		bot2, err = tgbotapi.NewBotAPI(cfg.Telegram.Bot2Token)
		if err != nil {
			logging.LogWarn("Failed to initialize bot 2 (continuing without it)", zap.Error(err))
		} else {
			logging.LogSuccess("Bot 2 authorized", zap.String("username", bot2.Self.UserName))
		}
	}

	return apiBot, bot1, bot2, nil
}

func startMonitors(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, client *flashnet.Client, apiBot, bot1, bot2 *tgbotapi.BotAPI) error {
	bigSalesBot := apiBot
	bigSalesChatID := cfg.Telegram.ApiBotChatID
	if bigSalesBot == nil || bigSalesChatID == "" {
		bigSalesBot = bot1
		bigSalesChatID = cfg.Telegram.BigSalesChatID
	}
	bigSalesMinBTCAmount := cfg.Telegram.BigSalesMinBTCAmount
	if bigSalesMinBTCAmount == 0 {
		bigSalesMinBTCAmount = 0.0025
	}

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

		var err error
		filteredTokensList, err = storage.LoadFilteredTokens()
		if err != nil {
			logging.LogWarn("Failed to load filtered tokens from file, using config", zap.Error(err))
			if len(cfg.Telegram.FilteredTokens) > 0 {
				filteredTokensList = cfg.Telegram.FilteredTokens
				if saveErr := storage.SaveFilteredTokens(filteredTokensList); saveErr != nil {
					logging.LogWarn("Failed to save filtered tokens from config to file", zap.Error(saveErr))
				}
			} else {
				filteredTokensList = []string{}
			}
		} else if len(filteredTokensList) == 0 && len(cfg.Telegram.FilteredTokens) > 0 {
			if saveErr := storage.SaveFilteredTokens(cfg.Telegram.FilteredTokens); saveErr != nil {
				logging.LogWarn("Failed to save filtered tokens from config to file", zap.Error(saveErr))
			} else {
				filteredTokensList = cfg.Telegram.FilteredTokens
			}
		}

		filteredMinBTCAmount = cfg.Telegram.FilteredMinBTCAmount
		if filteredMinBTCAmount == 0 {
			filteredMinBTCAmount = 0.01
		}
		logging.LogInfo("Filtered tokens monitor configured",
			zap.String("chatID", filteredChatID),
			zap.Int("tokensCount", len(filteredTokensList)),
			zap.Float64("minBTCAmount", filteredMinBTCAmount))

		if filteredBot != nil {
			shouldRunFilteredCommandHandler := true
			if bigSalesBot == filteredBot {
				if bigSalesChatID == cfg.Telegram.ApiBotChatID && cfg.Telegram.ApiBotChatID != "" {
					shouldRunFilteredCommandHandler = false
					logging.LogInfo("Skipping filtered command handler - same bot will use API chat handler",
						zap.String("filteredChatID", filteredChatID),
						zap.String("bigSalesChatID", bigSalesChatID))
				} else if filteredChatID == bigSalesChatID {
					shouldRunFilteredCommandHandler = false
					logging.LogInfo("Skipping filtered command handler - same bot and chat ID",
						zap.String("chatID", filteredChatID))
				}
			}

			if shouldRunFilteredCommandHandler {
			wg.Add(1)
			go func() {
				defer wg.Done()
				bots_monitor.RunCommandHandler(filteredBot, filteredChatID, client)
			}()
			}

			statsSendTime := cfg.Telegram.StatsSendTime
			if statsSendTime == "" {
				statsSendTime = "10:00"
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				bots_monitor.RunStatsMonitor(filteredBot, filteredChatID, statsSendTime)
			}()
		}
	}

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
			hotTokenSwapsCount = 6
		}
		hotTokenMinAddresses := cfg.Telegram.HotTokenMinAddresses
		if hotTokenMinAddresses == 0 {
			hotTokenMinAddresses = 3
		}
		checkInterval := cfg.App.CheckInterval
		if checkInterval == 0 {
			checkInterval = 30
		}

		if hotTokenBot != nil {
			logging.LogInfo("Hot token monitor configured",
				zap.Int("swapsCount", hotTokenSwapsCount),
				zap.Int("minAddresses", hotTokenMinAddresses),
				zap.Int("checkInterval", checkInterval),
				zap.String("chatID", cfg.Telegram.FilteredChatID))
			wg.Add(1)
			go func() {
				defer wg.Done()
				bots_monitor.RunHotTokenMonitor(hotTokenBot, client, cfg.Telegram.FilteredChatID, hotTokenSwapsCount, hotTokenMinAddresses, checkInterval)
			}()
		}
	}

	if bigSalesBot != nil && bigSalesChatID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bots_monitor.RunBigSalesBuysMonitor(bigSalesBot, client, bigSalesChatID, bigSalesMinBTCAmount, filteredBot, filteredChatID, filteredTokensList, filteredMinBTCAmount)
		}()

		// Start command handler for main chat (big sales chat)
		// If using the same bot, use one handler for all chats
		if bigSalesBot == filteredBot && filteredChatID != "" {
			// Use one handler for both chats
			apiChatID := ""
			if bigSalesChatID == cfg.Telegram.ApiBotChatID && cfg.Telegram.ApiBotChatID != "" {
				apiChatID = cfg.Telegram.ApiBotChatID
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Pass bigSalesChatID as filteredChatID to handle both chats
				handlerFilteredChatID := bigSalesChatID
				if filteredChatID != bigSalesChatID {
					handlerFilteredChatID = filteredChatID
				}
				logging.LogInfo("Using combined command handler for same bot",
					zap.String("bigSalesChatID", bigSalesChatID),
					zap.String("filteredChatID", filteredChatID),
					zap.String("handlerFilteredChatID", handlerFilteredChatID),
					zap.String("apiChatID", apiChatID))
				bots_monitor.RunCommandHandler(bigSalesBot, handlerFilteredChatID, client)
			}()
		} else if bigSalesChatID == cfg.Telegram.ApiBotChatID && cfg.Telegram.ApiBotChatID != "" {
			// Different bots or filteredChatID is empty - start separate handler for API chat
			wg.Add(1)
			go func() {
				defer wg.Done()
				bots_monitor.RunCommandHandler(bigSalesBot, bigSalesChatID, client)
			}()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bots_monitor.RunHoldersDynamicMonitor()
	}()

	return nil
}
