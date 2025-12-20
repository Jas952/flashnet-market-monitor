package luminex

// tokens from Luminex + (in and on

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"spark-wallet/internal/clients_api/flashnet"
	logging "spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

const (
	// LuminexAPIBaseURL - URL API Luminex
	LuminexAPIBaseURL = "https://api.luminex.io/spark/pool"
	TokenCacheFile    = "data_out/saved_ticket.json"
	// CacheTimeout - time in (5
	CacheTimeout = 5 * time.Minute
)

// TokenMetadataCache - for tokens
type TokenMetadataCache struct {
	mutex     sync.RWMutex
	cache     map[string]*TokenMetadata // poolLpPublicKey -> TokenMetadata
	cacheFile string
}

// TokenMetadata - token from API Luminex
type TokenMetadata struct {
	Name   string `json:"name"`
	Ticker string `json:"ticker"`
}

// LuminexPoolResponse - API Luminex
type LuminexPoolResponse struct {
	LpPublicKey    string               `json:"lpPublicKey"`
	AssetAAddress  string               `json:"assetAAddress"`
	AssetBAddress  string               `json:"assetBAddress"`
	TokenAMetadata LuminexTokenMetadata `json:"tokenAMetadata"`
	TokenBMetadata LuminexTokenMetadata `json:"tokenBMetadata"`
}

// LuminexTokenMetadataWithDecimals - token decimals
type LuminexTokenMetadataWithDecimals struct {
	LuminexTokenMetadata
	Decimals int `json:"decimals"`
}

// LuminexTokenMetadata - token from Luminex
type LuminexTokenMetadata struct {
	Name            string  `json:"name"`
	Ticker          string  `json:"ticker"`
	AggMarketcapUsd float64 `json:"agg_marketcap_usd"`
	AggPriceUsd     float64 `json:"agg_price_usd"`
	Decimals        int     `json:"decimals"`
}

// savedTicketsFile - for in file
type savedTicketsFile struct {
	Tickets map[string]string `json:"tickets"` // poolLpPublicKey -> "ticker:name"
}

var (
	tokenCache *TokenMetadataCache
	once       sync.Once
)

// getTokenCache tokens
func getTokenCache() *TokenMetadataCache {
	once.Do(func() {
		tokenCache = &TokenMetadataCache{
			cache:     make(map[string]*TokenMetadata),
			cacheFile: TokenCacheFile,
		}
		tokenCache.loadFromFile()
	})
	return tokenCache
}

// loadFromFile tokens from file
func (c *TokenMetadataCache) loadFromFile() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check
	dir := filepath.Dir(c.cacheFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogWarn("Failed to create cache directory", zap.Error(err))
		return
	}

	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			// file -
			return
		}
		logging.LogWarn("Failed to read token cache file", zap.Error(err))
		return
	}

	var saved savedTicketsFile
	if err := json.Unmarshal(data, &saved); err != nil {
		logging.LogWarn("Failed to parse token cache file", zap.Error(err))
		return
	}

	for poolKey, tickerName := range saved.Tickets {
		parts := strings.SplitN(tickerName, ":", 2)
		if len(parts) == 2 {
			c.cache[poolKey] = &TokenMetadata{
				Ticker: parts[0],
				Name:   parts[1],
			}
		}
	}

	logging.LogInfo("Loaded token cache from file", zap.Int("count", len(c.cache)))
}

// saveToFile tokens in file for
func (c *TokenMetadataCache) saveToFile() {
	c.mutex.RLock()
	cacheCopy := make(map[string]*TokenMetadata)
	for k, v := range c.cache {
		cacheCopy[k] = v
	}
	c.mutex.RUnlock()

	c.saveToFileUnlocked(cacheCopy)
}

func (c *TokenMetadataCache) getFromCache(poolLpPublicKey string) (*TokenMetadata, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	metadata, exists := c.cache[poolLpPublicKey]
	return metadata, exists
}

func (c *TokenMetadataCache) setToCache(poolLpPublicKey string, metadata *TokenMetadata) {
	c.mutex.Lock()
	c.cache[poolLpPublicKey] = metadata
	cacheCopy := make(map[string]*TokenMetadata)
	for k, v := range c.cache {
		cacheCopy[k] = v
	}
	c.mutex.Unlock()

	c.saveToFileUnlocked(cacheCopy)
}

// saveToFileUnlocked tokens in file
func (c *TokenMetadataCache) saveToFileUnlocked(cache map[string]*TokenMetadata) {
	saved := savedTicketsFile{
		Tickets: make(map[string]string),
	}

	// Save in "ticker:name"
	for poolKey, metadata := range cache {
		if metadata != nil {
			saved.Tickets[poolKey] = fmt.Sprintf("%s:%s", metadata.Ticker, metadata.Name)
		}
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		logging.LogWarn("Failed to marshal token cache", zap.Error(err))
		return
	}

	if err := os.WriteFile(c.cacheFile, data, 0644); err != nil {
		logging.LogWarn("Failed to save token cache file", zap.Error(err))
		return
	}

	logging.LogDebug("Saved token cache to file", zap.Int("count", len(saved.Tickets)))
}

// fetchFromAPI token from API Luminex
func fetchFromAPI(poolLpPublicKey string) (*TokenMetadata, error) {
	url := fmt.Sprintf("%s/%s", LuminexAPIBaseURL, poolLpPublicKey)

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

	var poolResp LuminexPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}

	// tokenBMetadata BTC (NativeTokenAddress)
	// tokenAMetadata token BTC)
	// Check addresses,
	var tokenMeta LuminexTokenMetadata

	if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
		// assetBAddress BTC, assetAAddress token
		tokenMeta = poolResp.TokenAMetadata
	} else if poolResp.AssetAAddress == flashnet.NativeTokenAddress {
		// assetAAddress BTC, assetBAddress token
		tokenMeta = poolResp.TokenBMetadata
	} else {
		// If tokenAMetadata
		tokenMeta = poolResp.TokenAMetadata
		if tokenMeta.Name == "" && tokenMeta.Ticker == "" {
			tokenMeta = poolResp.TokenBMetadata
		}
	}

	if tokenMeta.Name == "" && tokenMeta.Ticker == "" {
		return nil, fmt.Errorf("token metadata not found in API response")
	}

	return &TokenMetadata{
		Name:   tokenMeta.Name,
		Ticker: tokenMeta.Ticker,
	}, nil
}

// GetTokenMetadata token by poolLpPublicKey
// if - from API Luminex
func GetTokenMetadata(poolLpPublicKey string) *TokenMetadata {
	if poolLpPublicKey == "" {
		return nil
	}

	cache := getTokenCache()

	// Check
	if metadata, exists := cache.getFromCache(poolLpPublicKey); exists {
		return metadata
	}

	metadata, err := fetchFromAPI(poolLpPublicKey)
	if err != nil {
		// log API Luminex -
		// Return nil,
		return nil
	}

	cache.setToCache(poolLpPublicKey, metadata)

	return metadata
}

// GetPoolMarketCap token from Luminex API
// swap - swap for token (A or B)
// in USD or 0, if get
func GetPoolMarketCap(poolLpPublicKey string, swap flashnet.Swap) float64 {
	if poolLpPublicKey == "" {
		return 0
	}

	url := fmt.Sprintf("%s/%s", LuminexAPIBaseURL, poolLpPublicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.LogDebug("Failed to create request for pool marketcap", zap.Error(err))
		return 0
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		logging.LogDebug("Failed to fetch pool marketcap from Luminex API", zap.Error(err))
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.LogDebug("luminex pool API returned non-OK status", zap.Int("status", resp.StatusCode))
		return 0
	}

	var poolResp LuminexPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		logging.LogDebug("Failed to decode Luminex pool API response", zap.Error(err))
		return 0
	}

	// token (token, BTC)
	// Use addresses from swap for token BTC
	var marketcap float64

	// address token BTC) from swap
	var tokenAddress string
	if swap.AssetOutAddress == flashnet.NativeTokenAddress {
		// If get BTC, token (assetInAddress - token)
		tokenAddress = swap.AssetInAddress
	} else if swap.AssetInAddress == flashnet.NativeTokenAddress {
		// If BTC, get token (assetOutAddress - token)
		tokenAddress = swap.AssetOutAddress
	} else {
		// - use
		// PoolAssetBAddress BTC
		if swap.PoolAssetBAddress == flashnet.NativeTokenAddress {
			tokenAddress = swap.PoolAssetAAddress
		} else {
			tokenAddress = swap.PoolAssetBAddress
		}
	}

	// address token from pool
	if tokenAddress != "" {
		if tokenAddress == poolResp.AssetAAddress {
			marketcap = poolResp.TokenAMetadata.AggMarketcapUsd
		} else if tokenAddress == poolResp.AssetBAddress {
			marketcap = poolResp.TokenBMetadata.AggMarketcapUsd
		} else {
			// assetBAddress BTC in Luminex
			if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
				marketcap = poolResp.TokenAMetadata.AggMarketcapUsd
			} else {
				marketcap = poolResp.TokenBMetadata.AggMarketcapUsd
			}
		}
	}

	return marketcap
}

// GetTokenDecimals decimals token from Luminex API
// swap - swap for token (A or B, BTC)
// ticker - ticker token for
// decimals token or 8 (value by default), if get
func GetTokenDecimals(poolLpPublicKey string, swap flashnet.Swap, ticker string) int {
	if poolLpPublicKey == "" {
		return 8 // Default value
	}

	url := fmt.Sprintf("%s/%s", LuminexAPIBaseURL, poolLpPublicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.LogDebug("Failed to create request for token decimals", zap.Error(err))
		return 8
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		logging.LogDebug("Failed to fetch token decimals from Luminex API", zap.Error(err))
		return 8
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.LogDebug("Luminex pool API returned non-OK status for token decimals", zap.Int("status", resp.StatusCode))
		return 8
	}

	var poolResp LuminexPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		logging.LogDebug("Failed to decode Luminex pool API response for token decimals", zap.Error(err))
		return 8
	}

	// address token BTC) from swap GetPoolTokenPrice)
	var tokenAddress string
	if swap.AssetOutAddress == flashnet.NativeTokenAddress {
		tokenAddress = swap.AssetInAddress
	} else if swap.AssetInAddress == flashnet.NativeTokenAddress {
		tokenAddress = swap.AssetOutAddress
	} else {
		if swap.PoolAssetBAddress == flashnet.NativeTokenAddress {
			tokenAddress = swap.PoolAssetAAddress
		} else {
			tokenAddress = swap.PoolAssetBAddress
		}
	}

	// address token from pool
	var decimals int
	if tokenAddress != "" {
		if tokenAddress == poolResp.AssetAAddress {
			decimals = poolResp.TokenAMetadata.Decimals
			if ticker != "" && poolResp.TokenAMetadata.Ticker != ticker && poolResp.TokenBMetadata.Ticker == ticker {
				decimals = poolResp.TokenBMetadata.Decimals
			}
		} else if tokenAddress == poolResp.AssetBAddress {
			decimals = poolResp.TokenBMetadata.Decimals
			if ticker != "" && poolResp.TokenBMetadata.Ticker != ticker && poolResp.TokenAMetadata.Ticker == ticker {
				decimals = poolResp.TokenAMetadata.Decimals
			}
		} else {
			if ticker != "" {
				if poolResp.TokenAMetadata.Ticker == ticker && poolResp.AssetBAddress == flashnet.NativeTokenAddress {
					decimals = poolResp.TokenAMetadata.Decimals
				} else if poolResp.TokenBMetadata.Ticker == ticker && poolResp.AssetAAddress == flashnet.NativeTokenAddress {
					decimals = poolResp.TokenBMetadata.Decimals
				} else {
					if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
						decimals = poolResp.TokenAMetadata.Decimals
					} else {
						decimals = poolResp.TokenBMetadata.Decimals
					}
				}
			} else {
				if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
					decimals = poolResp.TokenAMetadata.Decimals
				} else {
					decimals = poolResp.TokenBMetadata.Decimals
				}
			}
		}
	}

	if decimals == 0 {
		return 8 // Default value
	}
	return decimals
}

// GetPoolTokenPrice token (agg_price_usd) from Luminex API
// swap - swap for token (A or B, BTC)
// ticker - ticker token for
// in USD or 0, if get
func GetPoolTokenPrice(poolLpPublicKey string, swap flashnet.Swap, ticker string) float64 {
	if poolLpPublicKey == "" {
		return 0
	}

	url := fmt.Sprintf("%s/%s", LuminexAPIBaseURL, poolLpPublicKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.LogDebug("Failed to create request for pool token price", zap.Error(err))
		return 0
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		logging.LogDebug("Failed to fetch pool token price from Luminex API", zap.Error(err))
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.LogDebug("Luminex pool API returned non-OK status for token price", zap.Int("status", resp.StatusCode))
		return 0
	}

	var poolResp LuminexPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		logging.LogDebug("Failed to decode Luminex pool API response for token price", zap.Error(err))
		return 0
	}

	// address token BTC) from swap
	var tokenAddress string
	if swap.AssetOutAddress == flashnet.NativeTokenAddress {
		// If get BTC, token (assetInAddress - token)
		tokenAddress = swap.AssetInAddress
	} else if swap.AssetInAddress == flashnet.NativeTokenAddress {
		// If BTC, get token (assetOutAddress - token)
		tokenAddress = swap.AssetOutAddress
	} else {
		// - use
		if swap.PoolAssetBAddress == flashnet.NativeTokenAddress {
			tokenAddress = swap.PoolAssetAAddress
		} else {
			tokenAddress = swap.PoolAssetBAddress
		}
	}

	// address token from pool
	var price float64
	if tokenAddress != "" {
		if tokenAddress == poolResp.AssetAAddress {
			price = poolResp.TokenAMetadata.AggPriceUsd
			if ticker != "" && poolResp.TokenAMetadata.Ticker != ticker {
				// If ticker tokenB
				if poolResp.TokenBMetadata.Ticker == ticker {
					price = poolResp.TokenBMetadata.AggPriceUsd
				}
			}
		} else if tokenAddress == poolResp.AssetBAddress {
			price = poolResp.TokenBMetadata.AggPriceUsd
			if ticker != "" && poolResp.TokenBMetadata.Ticker != ticker {
				// If ticker tokenA
				if poolResp.TokenAMetadata.Ticker == ticker {
					price = poolResp.TokenAMetadata.AggPriceUsd
				}
			}
		} else {
			// If addresses use and check ticker
			if ticker != "" {
				if poolResp.TokenAMetadata.Ticker == ticker && poolResp.AssetBAddress == flashnet.NativeTokenAddress {
					price = poolResp.TokenAMetadata.AggPriceUsd
				} else if poolResp.TokenBMetadata.Ticker == ticker && poolResp.AssetAAddress == flashnet.NativeTokenAddress {
					price = poolResp.TokenBMetadata.AggPriceUsd
				} else {
					if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
						price = poolResp.TokenAMetadata.AggPriceUsd
					} else {
						price = poolResp.TokenBMetadata.AggPriceUsd
					}
				}
			} else {
				if poolResp.AssetBAddress == flashnet.NativeTokenAddress {
					price = poolResp.TokenAMetadata.AggPriceUsd
				} else {
					price = poolResp.TokenBMetadata.AggPriceUsd
				}
			}
		}
	}

	return price
}

// GetWalletTokenHolding holding token wallet
// ticker - ticker token
// count tokens and in USD
// If token "null" for
func GetWalletTokenHolding(publicKey string, poolLpPublicKey string, swap flashnet.Swap, ticker string) (string, string) {
	if publicKey == "" || poolLpPublicKey == "" || ticker == "" {
		return "null", ""
	}

	balanceResp, err := GetWalletTokensBalance(publicKey)
	if err != nil {
		logging.LogDebug("Failed to get wallet tokens balance", zap.String("publicKey", publicKey), zap.Error(err))
		return "null", ""
	}

	// token by ticker in wallet
	var tokenBalance *WalletToken
	for i := range balanceResp.Tokens {
		if balanceResp.Tokens[i].Ticker == ticker {
			tokenBalance = &balanceResp.Tokens[i]
			break
		}
	}

	if tokenBalance == nil {
		// token wallet
		return "null", ""
	}

	// Parse balance
	var balanceValue float64
	fmt.Sscanf(tokenBalance.Balance, "%f", &balanceValue)

	// on 10^decimals
	decimalsMultiplier := 1.0
	for i := 0; i < tokenBalance.Decimals; i++ {
		decimalsMultiplier *= 10
	}
	tokenAmount := balanceValue / decimalsMultiplier

	tokenAmountStr := formatTokenAmount(tokenAmount)

	tokenPrice := GetPoolTokenPrice(poolLpPublicKey, swap, ticker)

	if tokenPrice == 0 {
		// If get return count
		return tokenAmountStr, ""
	}

	// Calculate in USD
	valueUsd := tokenAmount * tokenPrice

	// Format
	valueUsdStr := formatMarketCapUsd(valueUsd)

	return tokenAmountStr, valueUsdStr
}

// formatMarketCapUsd in USD in
func formatMarketCapUsd(value float64) string {
	if value == 0 {
		return ""
	}

	if value >= 1e9 {
		val := value / 1e9
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", val), "0"), ".")
		return fmt.Sprintf("$%sB", formatted)
	} else if value >= 1e6 {
		val := value / 1e6
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", val), "0"), ".")
		return fmt.Sprintf("$%sM", formatted)
	} else if value >= 1e3 {
		val := value / 1e3
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", val), "0"), ".")
		return fmt.Sprintf("$%sK", formatted)
	} else {
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
		return fmt.Sprintf("$%s", formatted)
	}
}

// formatTokenAmount count tokens in (1.1M, 2.2K and ..)
func formatTokenAmount(amount float64) string {
	if amount == 0 {
		return "0"
	}

	if amount >= 1e9 {
		value := amount / 1e9
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sB", formatted)
	} else if amount >= 1e6 {
		value := amount / 1e6
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sM", formatted)
	} else if amount >= 1e3 {
		value := amount / 1e3
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
		return fmt.Sprintf("%sK", formatted)
	} else {
		// - 2 or if
		if amount == float64(int64(amount)) {
			return fmt.Sprintf("%.0f", amount)
		}
		formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", amount), "0"), ".")
		return formatted
	}
}
