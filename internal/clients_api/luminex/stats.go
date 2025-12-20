package luminex

// Package system_works contains for from API Luminex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	storage "spark-wallet/internal/infra/fs"
	logging "spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

const (
	// LuminexStatsAPIBaseURL - URL API Luminex for
	LuminexStatsAPIBaseURL = "https://api.luminex.io/spark/stats"
	// LuminexTokensAPIBaseURL - URL API Luminex for tokens
	LuminexTokensAPIBaseURL = "https://api.luminex.io/spark/tokens-with-pools"
	// LuminexPoolStatsAPIBaseURL - URL API Luminex for pool
	LuminexPoolStatsAPIBaseURL = "https://api.luminex.io/spark/pools"
)

// StatsResponse - API Luminex for
type StatsResponse struct {
	TotalTokens       int     `json:"total_tokens"`
	TotalMarketCapUSD float64 `json:"total_market_cap_usd"`
	TotalVolume24HUSD float64 `json:"total_volume_24h_usd"`
	TotalTVLUSD       float64 `json:"total_tvl_usd"`
	TotalPools        int     `json:"total_pools"`
}

// TokenInfo - from API
type TokenInfo struct {
	Ticker         string  `json:"ticker"`
	Volume24HUSD   float64 `json:"agg_volume_24h_usd"`
	MarketCapUSD   float64 `json:"agg_marketcap_usd"`
	PriceChange24H float64 `json:"agg_price_change_24h"`
}

// TokensResponse - API Luminex for tokens
type TokensResponse []TokenInfo

// GetStats from API Luminex
func GetStats() (*StatsResponse, error) {
	url := LuminexStatsAPIBaseURL

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
		return nil, fmt.Errorf("failed to fetch from Luminex Stats API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex Stats API returned status %d", resp.StatusCode)
	}

	var statsResp StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex Stats API response: %w", err)
	}

	return &statsResp, nil
}

// GetTopTokens -5 tokens by 24 from API Luminex
// tokens, BTC and USDB count
func GetTopTokens(limit int) ([]TokenInfo, error) {
	if limit <= 0 {
		limit = 5
	}

	// tokens (limit + 2), BTC and USDB count
	// If BTC and USDB in
	requestLimit := limit + 2

	// URL
	url := fmt.Sprintf("%s?offset=0&limit=%d&sort_by=agg_volume_24h_usd&order=desc",
		LuminexTokensAPIBaseURL, requestLimit)

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
		return nil, fmt.Errorf("failed to fetch from Luminex Tokens API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex Tokens API returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	bodyPreview := string(bodyBytes)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "..."
	}
	logging.LogDebug("Luminex Tokens API response preview",
		zap.String("preview", bodyPreview),
		zap.Int("bodyLength", len(bodyBytes)))

	var tokensResp []map[string]interface{}

	if err := json.Unmarshal(bodyBytes, &tokensResp); err != nil {
		logging.LogDebug("Failed to parse as array, trying as object", zap.Error(err))

		var objResp map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &objResp); err != nil {
			return nil, fmt.Errorf("failed to decode Luminex Tokens API response (neither array nor object): %w", err)
		}

		keys := make([]string, 0, len(objResp))
		for k := range objResp {
			keys = append(keys, k)
		}
		logging.LogDebug("Parsed as object, looking for array", zap.Strings("keys", keys))

		found := false
		if data, ok := objResp["data"].([]interface{}); ok {
			logging.LogDebug("Found array in 'data' field", zap.Int("length", len(data)))
			tokensResp = make([]map[string]interface{}, len(data))
			for i, item := range data {
				if m, ok := item.(map[string]interface{}); ok {
					tokensResp[i] = m
				}
			}
			found = true
		} else if results, ok := objResp["results"].([]interface{}); ok {
			logging.LogDebug("Found array in 'results' field", zap.Int("length", len(results)))
			tokensResp = make([]map[string]interface{}, len(results))
			for i, item := range results {
				if m, ok := item.(map[string]interface{}); ok {
					tokensResp[i] = m
				}
			}
			found = true
		} else if items, ok := objResp["items"].([]interface{}); ok {
			logging.LogDebug("Found array in 'items' field", zap.Int("length", len(items)))
			tokensResp = make([]map[string]interface{}, len(items))
			for i, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					tokensResp[i] = m
				}
			}
			found = true
		} else {
			for key, v := range objResp {
				if arr, ok := v.([]interface{}); ok {
					logging.LogDebug("Found array in field", zap.String("field", key), zap.Int("length", len(arr)))
					tokensResp = make([]map[string]interface{}, len(arr))
					for i, item := range arr {
						if m, ok := item.(map[string]interface{}); ok {
							tokensResp[i] = m
						}
					}
					found = true
					break
				}
			}
		}

		if !found || len(tokensResp) == 0 {
			return nil, fmt.Errorf("failed to find tokens array in API response (object keys: %v)", keys)
		}
	} else {
		logging.LogDebug("Successfully parsed as array", zap.Int("length", len(tokensResp)))
	}

	var topTokens []TokenInfo
	for _, token := range tokensResp {
		// Check
		if len(topTokens) >= limit {
			break
		}

		ticker, _ := token["ticker"].(string)
		if ticker == "" {
			continue
		}

		// BTC and USDB from
		if ticker == "BTC" || ticker == "USDB" {
			continue
		}

		getFloat64 := func(key string) float64 {
			val, ok := token[key]
			if !ok {
				return 0
			}
			switch v := val.(type) {
			case float64:
				return v
			case string:
				var result float64
				if _, err := fmt.Sscanf(v, "%f", &result); err == nil {
					return result
				}
				return 0
			}
			return 0
		}

		volume24H := getFloat64("agg_volume_24h_usd")
		marketCap := getFloat64("agg_marketcap_usd")
		priceChange := getFloat64("agg_price_change_24h")

		topTokens = append(topTokens, TokenInfo{
			Ticker:         ticker,
			Volume24HUSD:   volume24H,
			MarketCapUSD:   marketCap,
			PriceChange24H: priceChange,
		})
	}

	return topTokens, nil
}

// PoolStatsResponse - API Luminex for pool
type PoolStatsResponse struct {
	Txns        int    `json:"txns"`
	Buys        int    `json:"buys"`
	Sells       int    `json:"sells"`
	TotalVolume string `json:"totalVolume"`
	BuyVolume   string `json:"buyVolume"`
	SellVolume  string `json:"sellVolume"`
	CurrentTime string `json:"currentTime"`
}

// GetPoolStats pool 24 from API Luminex
func GetPoolStats(poolLpPublicKey string) (*PoolStatsResponse, error) {
	if poolLpPublicKey == "" {
		return nil, fmt.Errorf("poolLpPublicKey is required")
	}

	// URL
	url := fmt.Sprintf("%s/%s/stats?timeframe=24h", LuminexPoolStatsAPIBaseURL, poolLpPublicKey)

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
		return nil, fmt.Errorf("failed to fetch from Luminex Pool Stats API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("luminex Pool Stats API returned status %d", resp.StatusCode)
	}

	var poolStatsResp PoolStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolStatsResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex Pool Stats API response: %w", err)
	}

	return &poolStatsResp, nil
}

// GetPoolLpPublicKeyForTicker poolLpPublicKey for from saved_ticket.json
// FindPoolLpPublicKeyByTicker from filtered_tokens.go
func GetPoolLpPublicKeyForTicker(ticker string) (string, error) {
	// Use from filtered_tokens.go
	return storage.FindPoolLpPublicKeyByTicker(ticker)
}

// FormatUSDValue value in USD M and K
// "1.2M" or "300.5K"
func FormatUSDValue(value float64) string {
	if value >= 1e6 {
		val := value / 1e6
		formatted := fmt.Sprintf("%.1f", val)
		formatted = strings.TrimRight(formatted, "0")
		formatted = strings.TrimRight(formatted, ".")
		return formatted + "M"
	} else if value >= 1e3 {
		val := value / 1e3
		formatted := fmt.Sprintf("%.1f", val)
		formatted = strings.TrimRight(formatted, "0")
		formatted = strings.TrimRight(formatted, ".")
		return formatted + "K"
	}
	formatted := fmt.Sprintf("%.0f", value)
	return formatted
}

type StatsDataEntry struct {
	Date              string  `json:"date"` // date in YYYY-MM-DD
	TotalTokens       int     `json:"total_tokens"`
	TotalMarketCapUSD float64 `json:"total_market_cap_usd"`
	TotalVolume24HUSD float64 `json:"total_volume_24h_usd"`
	TotalTVLUSD       float64 `json:"total_tvl_usd"`
	TotalPools        int     `json:"total_pools"`
	Check             bool    `json:"check"`
}

// StatsData - for in
type StatsData struct {
	Entries []StatsDataEntry `json:"entries"`
}

// SaveStatsData data in file stats.json
func SaveStatsData(stats *StatsResponse, check bool) error {
	dataOutDir := filepath.Join("data_out", "telegram_out")
	if err := os.MkdirAll(dataOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create telegram_out directory: %w", err)
	}

	filename := filepath.Join(dataOutDir, "stats.json")

	existingData, err := LoadStatsData()
	if err != nil {
		existingData = &StatsData{Entries: []StatsDataEntry{}}
	}

	// Get
	currentDate := time.Now().Format("2006-01-02")

	// Check,
	found := false
	for i := range existingData.Entries {
		if existingData.Entries[i].Date == currentDate {
			// Update
			// - (10:00) check = true
			// - /stats check = true if
			existingData.Entries[i] = StatsDataEntry{
				Date:              currentDate,
				TotalTokens:       stats.TotalTokens,
				TotalMarketCapUSD: stats.TotalMarketCapUSD,
				TotalVolume24HUSD: stats.TotalVolume24HUSD,
				TotalTVLUSD:       stats.TotalTVLUSD,
				TotalPools:        stats.TotalPools,
				Check:             check,
			}
			found = true
			break
		}
	}

	if !found {
		existingData.Entries = append(existingData.Entries, StatsDataEntry{
			Date:              currentDate,
			TotalTokens:       stats.TotalTokens,
			TotalMarketCapUSD: stats.TotalMarketCapUSD,
			TotalVolume24HUSD: stats.TotalVolume24HUSD,
			TotalTVLUSD:       stats.TotalTVLUSD,
			TotalPools:        stats.TotalPools,
			Check:             check,
		})
	}

	dataBytes, err := json.MarshalIndent(existingData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats data: %w", err)
	}

	if err := os.WriteFile(filename, dataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write stats file: %w", err)
	}

	logging.LogDebug("Stats data saved",
		zap.String("date", currentDate),
		zap.Bool("check", check),
		zap.Float64("tvl", stats.TotalTVLUSD),
		zap.Float64("volume24h", stats.TotalVolume24HUSD))

	return nil
}

// LoadStatsData data from file stats.json
func LoadStatsData() (*StatsData, error) {
	filename := filepath.Join("data_out", "telegram_out", "stats.json")

	// Check, file
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return &StatsData{Entries: []StatsDataEntry{}}, nil
	}

	dataBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read stats file: %w", err)
	}

	var statsData StatsData
	if err := json.Unmarshal(dataBytes, &statsData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats data: %w", err)
	}

	return &statsData, nil
}

// IsStatsCheckedToday
func IsStatsCheckedToday() (bool, error) {
	statsData, err := LoadStatsData()
	if err != nil {
		return false, err
	}

	currentDate := time.Now().Format("2006-01-02")

	for _, entry := range statsData.Entries {
		if entry.Date == currentDate {
			return entry.Check, nil
		}
	}

	return false, nil
}
