package luminex

// Package system_works contains for wallet from API Luminex

import (
	"encoding/json"
	"fmt"
	"net/http"
	logging "spark-wallet/internal/infra/log"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// LuminexAddressAPIBaseURL - URL API Luminex for wallet
	LuminexAddressAPIBaseURL = "https://api.luminex.io/spark/address"
	// LuminexProfilesAPIBaseURL - URL API Luminex for
	LuminexProfilesAPIBaseURL = "https://api.luminex.io/spark-users/profiles"
	// LuminexPoolAPIBaseURL - URL API Luminex for
	LuminexPoolAPIBaseURL = "https://api.luminex.io/spark/pool"
)

// WalletBalanceResponse - API Luminex for wallet
type WalletBalanceResponse struct {
	SparkAddress     string        `json:"sparkAddress"`
	PublicKey        string        `json:"publicKey"`
	Balance          WalletBalance `json:"balance"`
	TotalValueUsd    float64       `json:"totalValueUsd"`
	TransactionCount int           `json:"transactionCount"`
	TokenCount       int           `json:"tokenCount"`
	Tokens           []WalletToken `json:"tokens"`
}

// WalletToken - token in wallet
type WalletToken struct {
	TokenIdentifier string  `json:"tokenIdentifier"`
	TokenAddress    string  `json:"tokenAddress"`
	Name            string  `json:"name"`
	Ticker          string  `json:"ticker"`
	Decimals        int     `json:"decimals"`
	Balance         string  `json:"balance"`
	ValueUsd        float64 `json:"valueUsd"`
}

type WalletBalance struct {
	BtcHardBalanceSats int64   `json:"btcHardBalanceSats"`
	BtcSoftBalanceSats int64   `json:"btcSoftBalanceSats"`
	BtcValueUsdHard    float64 `json:"btcValueUsdHard"`
	BtcValueUsdSoft    float64 `json:"btcValueUsdSoft"`
	TotalTokenValueUsd float64 `json:"totalTokenValueUsd"`
}

var (
	balanceCache      = make(map[string]*WalletBalanceResponse)
	balanceCacheMutex sync.RWMutex

	usernameCache      = make(map[string]string) // pubkey -> username
	usernameCacheMutex sync.RWMutex
)

// UserProfileResponse - API Luminex for
type UserProfileResponse struct {
	Data []UserProfile `json:"data"`
}

// UserProfile -
type UserProfile struct {
	Pubkey   string `json:"pubkey"`
	Username string `json:"username"`
	ImageURL string `json:"image_url"`
}

// GetWalletBalance balance wallet by
func GetWalletBalance(publicKey string) (*WalletBalanceResponse, error) {
	if publicKey == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	// Check
	balanceCacheMutex.RLock()
	if cached, exists := balanceCache[publicKey]; exists {
		balanceCacheMutex.RUnlock()
		return cached, nil
	}
	balanceCacheMutex.RUnlock()

	url := fmt.Sprintf("%s/%s", LuminexAddressAPIBaseURL, publicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// create Cloudflare)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Luminex API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex API returned status %d", resp.StatusCode)
	}

	var balanceResp WalletBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}

	balanceCacheMutex.Lock()
	balanceCache[publicKey] = &balanceResp
	balanceCacheMutex.Unlock()

	return &balanceResp, nil
}

// FormatBTCFromSats in BTC
func FormatBTCFromSats(sats int64) string {
	btc := float64(sats) / 1e8
	return fmt.Sprintf("%.8f", btc)
}

// GetWalletUsername (username) by
// username or if or error
func GetWalletUsername(publicKey string) string {
	if publicKey == "" {
		return ""
	}

	// Check
	usernameCacheMutex.RLock()
	if username, exists := usernameCache[publicKey]; exists {
		usernameCacheMutex.RUnlock()
		return username
	}
	usernameCacheMutex.RUnlock()

	url := fmt.Sprintf("%s?pubkeys=%s", LuminexProfilesAPIBaseURL, publicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// create Cloudflare)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.LogWarn("Failed to create request for wallet username", zap.String("publicKey", publicKey), zap.Error(err))
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		logging.LogWarn("Failed to fetch wallet username from Luminex API", zap.String("publicKey", publicKey), zap.Error(err))
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.LogWarn("Luminex profiles API returned non-OK status", zap.String("publicKey", publicKey), zap.Int("status", resp.StatusCode))
		return ""
	}

	var profileResp UserProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
		logging.LogWarn("Failed to decode Luminex profiles API response", zap.String("publicKey", publicKey), zap.Error(err))
		return ""
	}

	var username string
	for _, profile := range profileResp.Data {
		if profile.Pubkey == publicKey && profile.Username != "" {
			username = profile.Username
			break
		}
	}

	// Save in if username
	usernameCacheMutex.Lock()
	usernameCache[publicKey] = username
	usernameCacheMutex.Unlock()

	return username
}

// GetWalletTokensBalance balance wallet by
func GetWalletTokensBalance(publicKey string) (*WalletBalanceResponse, error) {
	if publicKey == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	url := fmt.Sprintf("%s/%s", LuminexAddressAPIBaseURL, publicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// create Cloudflare)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Luminex API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex API returned status %d", resp.StatusCode)
	}

	var balanceResp WalletBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}

	return &balanceResp, nil
}

// PoolResponse - API Luminex for
type PoolResponse struct {
	LpPublicKey    string            `json:"lpPublicKey"`
	TokenAMetadata PoolTokenMetadata `json:"tokenAMetadata"`
	TokenBMetadata PoolTokenMetadata `json:"tokenBMetadata"`
}

// PoolTokenMetadata - token from API Luminex Pool
type PoolTokenMetadata struct {
	TotalSupply string `json:"total_supply"` // token
	Decimals    int    `json:"decimals"`
	Ticker      string `json:"ticker"`
	IconURL     string `json:"icon_url"` // URL token
}

// GetPoolTotalSupply total_supply token from API Luminex by poolLpPublicKey
// total_supply in and count decimals
func GetPoolTotalSupply(poolLpPublicKey string) (string, int, error) {
	if poolLpPublicKey == "" {
		return "", 0, fmt.Errorf("poolLpPublicKey is empty")
	}

	url := fmt.Sprintf("%s/%s", LuminexPoolAPIBaseURL, poolLpPublicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// create Cloudflare)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch from Luminex Pool API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("luminex Pool API returned status %d", resp.StatusCode)
	}

	var poolResp PoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode Luminex Pool API response: %w", err)
	}

	// Return total_supply from tokenAMetadata (token A - token, token B - BTC)
	if poolResp.TokenAMetadata.TotalSupply != "" {
		return poolResp.TokenAMetadata.TotalSupply, poolResp.TokenAMetadata.Decimals, nil
	}

	return "", 0, fmt.Errorf("total_supply not found in pool response")
}
