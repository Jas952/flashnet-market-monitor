package system_works

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config -
type Config struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
	Flashnet FlashnetConfig `mapstructure:"flashnet"`
	App      AppConfig      `mapstructure:"app"`
}

type TelegramConfig struct {
	Bot1Token            string   `mapstructure:"bot1_token"`
	Bot2Token            string   `mapstructure:"bot2_token"`
	ApiBotToken          string   `mapstructure:"api_bot_token"` // API- for
	BigSalesChatID       string   `mapstructure:"big_sales_chat_id"`
	ApiBotChatID         string   `mapstructure:"api_bot_chat_id"`          // Chat ID for API-
	FilteredChatID       string   `mapstructure:"filtered_chat_id"`         // Chat ID for tokens
	FilteredTokens       []string `mapstructure:"filtered_tokens"`          // poolLpPublicKey from YAML or from .env)
	BigSalesMinBTCAmount float64  `mapstructure:"big_sales_min_btc_amount"` // amount for (by default 0.0025)
	FilteredMinBTCAmount float64  `mapstructure:"filtered_min_btc_amount"`  // amount for (by default 0.01)
	StatsSendTime        string   `mapstructure:"stats_send_time"`          // time "10:00", by default "10:00")
	HotTokenSwapsCount   int      `mapstructure:"hot_token_swaps_count"`    // count for token (by default 6)
	HotTokenMinAddresses int      `mapstructure:"hot_token_min_addresses"`  // count for token (by default 3)
}

// FlashnetConfig - Flashnet API
type FlashnetConfig struct {
	Network        string `mapstructure:"network"`
	PublicKey      string `mapstructure:"public_key"`
	RequestTimeout int    `mapstructure:"request_timeout"`
	MaxRetries     int    `mapstructure:"max_retries"`
}

// AppConfig -
type AppConfig struct {
	DataDir         string `mapstructure:"data_dir"`
	CheckInterval   int    `mapstructure:"check_interval"`
	MaxResponseSize int64  `mapstructure:"max_response_size"`
}

// LoadConfig from env, and
// 1. by default
// 2. config.yaml
// 3. .env file
func LoadConfig() (*Config, error) {
	// Load .env file godotenv for
	godotenv.Load(".env")

	v := viper.New()

	setDefaults(v)

	// Load from config.yaml (if -
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.ReadInConfig() // error, if file

	// monitoring.* -> telegram.*
	if v.IsSet("monitoring.big_sales_min_btc_amount") {
		v.Set("telegram.big_sales_min_btc_amount", v.Get("monitoring.big_sales_min_btc_amount"))
	}
	if v.IsSet("monitoring.filtered_min_btc_amount") {
		v.Set("telegram.filtered_min_btc_amount", v.Get("monitoring.filtered_min_btc_amount"))
	}
	if v.IsSet("monitoring.stats_send_time") {
		v.Set("telegram.stats_send_time", v.Get("monitoring.stats_send_time"))
	}
	if v.IsSet("monitoring.hot_token.swaps_count") {
		v.Set("telegram.hot_token_swaps_count", v.Get("monitoring.hot_token.swaps_count"))
	}
	if v.IsSet("monitoring.hot_token.min_addresses") {
		v.Set("telegram.hot_token_min_addresses", v.Get("monitoring.hot_token.min_addresses"))
	}

	// Load from .env file (if -
	v.SetConfigType("env")
	v.SetConfigFile(".env")
	v.ReadInConfig() // error, if file

	v.AutomaticEnv()

	setupEnvAliases(v)

	// Configure
	setupFlags(v)

	// Create
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// FilteredTokens from in (for .env)
	// If from .env Viper in
	if filteredTokensRaw := v.Get("telegram.filtered_tokens"); filteredTokensRaw != nil {
		switch v := filteredTokensRaw.(type) {
		case string:
			// If (from .env), parse
			if v != "" {
				config.Telegram.FilteredTokens = strings.Split(v, ",")
				for i, token := range config.Telegram.FilteredTokens {
					config.Telegram.FilteredTokens[i] = strings.TrimSpace(token)
				}
			} else {
				config.Telegram.FilteredTokens = []string{}
			}
		case []string:
			// If (from YAML), use
			config.Telegram.FilteredTokens = v
		case []interface{}:
			// If Viper []interface{}, in []string
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, strings.TrimSpace(str))
				}
			}
			config.Telegram.FilteredTokens = result
		}
	}

	// Check
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setupEnvAliases(v *viper.Viper) {
	// and SPARK_)
	// TELEGRAM_BOT1_TOKEN -> telegram.bot1_token

	// Telegram -
	v.BindEnv("telegram.bot1_token", "TELEGRAM_BOT1_TOKEN")
	v.BindEnv("telegram.bot2_token", "TELEGRAM_BOT2_TOKEN")
	v.BindEnv("telegram.api_bot_token", "API_BOT_TOKEN")
	v.BindEnv("telegram.big_sales_chat_id", "BIG_SALES_CHAT_ID")
	v.BindEnv("telegram.api_bot_chat_id", "API_BOT_CHAT_ID")
	v.BindEnv("telegram.filtered_chat_id", "FILTERED_CHAT_ID")
	v.BindEnv("telegram.filtered_tokens", "FILTERED_TOKENS")
	v.BindEnv("telegram.big_sales_min_btc_amount", "BIG_SALES_MIN_BTC_AMOUNT")
	v.BindEnv("telegram.filtered_min_btc_amount", "FILTERED_MIN_BTC_AMOUNT")
	v.BindEnv("telegram.stats_send_time", "STATS_SEND_TIME")
	v.BindEnv("telegram.hot_token_swaps_count", "HOT_TOKEN_SWAPS_COUNT")
	v.BindEnv("telegram.hot_token_min_addresses", "HOT_TOKEN_MIN_ADDRESSES")

	// Flashnet -
	v.BindEnv("flashnet.network", "NETWORK")
	v.BindEnv("flashnet.public_key", "PUBLIC_KEY")
	v.BindEnv("flashnet.request_timeout", "SPARK_FLASHNET_REQUEST_TIMEOUT")
	v.BindEnv("flashnet.max_retries", "SPARK_FLASHNET_MAX_RETRIES")

	// App -
	v.BindEnv("app.data_dir", "SPARK_APP_DATA_DIR")
	v.BindEnv("app.check_interval", "SPARK_APP_CHECK_INTERVAL")
	v.BindEnv("app.max_response_size", "SPARK_APP_MAX_RESPONSE_SIZE")
}

// setDefaults by default
func setDefaults(v *viper.Viper) {
	// Telegram
	v.SetDefault("telegram.bot1_token", "")
	v.SetDefault("telegram.bot2_token", "")
	v.SetDefault("telegram.api_bot_token", "")
	v.SetDefault("telegram.big_sales_chat_id", "")
	v.SetDefault("telegram.api_bot_chat_id", "")
	v.SetDefault("telegram.filtered_chat_id", "")
	v.SetDefault("telegram.filtered_tokens", []string{})
	v.SetDefault("telegram.big_sales_min_btc_amount", 0.0025) // 0.0025 BTC by default
	v.SetDefault("telegram.filtered_min_btc_amount", 0.01)    // 0.01 BTC by default
	v.SetDefault("telegram.stats_send_time", "10:00")         // 10:00 by default
	v.SetDefault("telegram.hot_token_swaps_count", 6)         // 6 by default
	v.SetDefault("telegram.hot_token_min_addresses", 3)       // 3 addresses by default

	// Flashnet
	v.SetDefault("flashnet.network", "mainnet")
	v.SetDefault("flashnet.public_key", "")
	v.SetDefault("flashnet.request_timeout", 30)
	v.SetDefault("flashnet.max_retries", 3)

	// App
	v.SetDefault("app.data_dir", "data_in")
	v.SetDefault("app.check_interval", 30)
	v.SetDefault("app.max_response_size", 10*1024*1024) // 10MB
}

func setupFlags(v *viper.Viper) {
	// Telegram
	pflag.String("telegram.bot1_token", "", "Telegram Bot 1 token (env: SPARK_TELEGRAM_BOT1_TOKEN)")
	pflag.String("telegram.bot2_token", "", "Telegram Bot 2 token (env: SPARK_TELEGRAM_BOT2_TOKEN)")
	pflag.String("telegram.api_bot_token", "", "API Bot token for swap notifications (env: API_BOT_TOKEN)")
	pflag.String("telegram.big_sales_chat_id", "", "Big Sales Chat ID (env: SPARK_TELEGRAM_BIG_SALES_CHAT_ID)")
	pflag.String("telegram.api_bot_chat_id", "", "API Bot Chat ID (env: API_BOT_CHAT_ID)")
	pflag.String("telegram.filtered_chat_id", "", "Filtered tokens Chat ID (env: FILTERED_CHAT_ID)")
	pflag.String("telegram.filtered_tokens", "", "Comma-separated list of poolLpPublicKey for filtered chat (env: FILTERED_TOKENS)")
	pflag.Float64("telegram.big_sales_min_btc_amount", 0.0025, "Minimum BTC amount for big sales chat (env: BIG_SALES_MIN_BTC_AMOUNT)")
	pflag.Float64("telegram.filtered_min_btc_amount", 0.01, "Minimum BTC amount for filtered chat (env: FILTERED_MIN_BTC_AMOUNT)")
	pflag.String("telegram.stats_send_time", "10:00", "Time to send stats report (format: HH:MM, env: STATS_SEND_TIME)")
	pflag.Int("telegram.hot_token_swaps_count", 6, "Number of swaps to check for hot token (env: HOT_TOKEN_SWAPS_COUNT)")
	pflag.Int("telegram.hot_token_min_addresses", 3, "Minimum number of different addresses for hot token (env: HOT_TOKEN_MIN_ADDRESSES)")

	// Flashnet
	pflag.String("flashnet.network", "mainnet", "Network: mainnet or testnet (env: SPARK_FLASHNET_NETWORK)")
	pflag.String("flashnet.public_key", "", "Public key for API auth (env: SPARK_FLASHNET_PUBLIC_KEY)")
	pflag.Int("flashnet.request_timeout", 30, "Request timeout in seconds (env: SPARK_FLASHNET_REQUEST_TIMEOUT)")
	pflag.Int("flashnet.max_retries", 3, "Max retries for failed requests (env: SPARK_FLASHNET_MAX_RETRIES)")

	// App
	pflag.String("app.data_dir", "data_in", "Data directory (env: SPARK_APP_DATA_DIR)")
	pflag.Int("app.check_interval", 30, "Check interval in seconds (env: SPARK_APP_CHECK_INTERVAL)")
	pflag.Int64("app.max_response_size", 10*1024*1024, "Max response size in bytes (env: SPARK_APP_MAX_RESPONSE_SIZE)")

	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)
}

func validateConfig(cfg *Config) error {
	// Check, (Bot1Token or ApiBotToken)
	if cfg.Telegram.Bot1Token == "" && cfg.Telegram.ApiBotToken == "" {
		return fmt.Errorf("at least one bot token is required: telegram.bot1_token or telegram.api_bot_token")
	}

	// Check, for Big Sales (BigSalesChatID or ApiBotChatID)
	if cfg.Telegram.BigSalesChatID == "" && cfg.Telegram.ApiBotChatID == "" {
		return fmt.Errorf("at least one big sales chat is required: telegram.big_sales_chat_id or telegram.api_bot_chat_id")
	}

	return nil
}
