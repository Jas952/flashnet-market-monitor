package hot_token

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"spark-wallet/internal/clients_api/flashnet"
	"spark-wallet/internal/clients_api/luminex"
	"spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

// LuminexFullPoolResponse - API Luminex response for pool (extended metadata).
type LuminexFullPoolResponse struct {
	LpPublicKey               string                   `json:"lpPublicKey"`
	CurveType                 string                   `json:"curveType"`
	LpFeeBps                  int                      `json:"lpFeeBps"`
	HostName                  string                   `json:"hostName"`
	HostFeeBps                int                      `json:"hostFeeBps"`
	AssetAAddress             string                   `json:"assetAAddress"`
	AssetBAddress             string                   `json:"assetBAddress"`
	AssetAReserve             string                   `json:"assetAReserve"`
	AssetBReserve             string                   `json:"assetBReserve"`
	CurrentPriceAInB          string                   `json:"currentPriceAInB"`
	TvlAssetB                 string                   `json:"tvlAssetB"`
	Volume24hAssetB           string                   `json:"volume24hAssetB"`
	PriceChangePercent24h     string                   `json:"priceChangePercent24h"`
	BondingProgressPercent    string                   `json:"bondingProgressPercent"`
	InitialReserveA           string                   `json:"initialReserveA"`
	VirtualReserveA           string                   `json:"virtualReserveA"`
	VirtualReserveB           string                   `json:"virtualReserveB"`
	GraduationThresholdAmount string                   `json:"graduationThresholdAmount"`
	CreatedAt                 string                   `json:"createdAt"`
	UpdatedAt                 string                   `json:"updatedAt"`
	TokenAMetadata            LuminexFullTokenMetadata `json:"tokenAMetadata"`
	TokenBMetadata            LuminexFullTokenMetadata `json:"tokenBMetadata"`
	Extra                     LuminexPoolExtra         `json:"extra"`
}

// LuminexFullTokenMetadata - token metadata from Luminex.
type LuminexFullTokenMetadata struct {
	ID                 int      `json:"id"`
	Pubkey             string   `json:"pubkey"`
	TokenIdentifier    string   `json:"token_identifier"`
	TokenAddress       string   `json:"token_address"`
	Name               string   `json:"name"`
	Ticker             string   `json:"ticker"`
	Decimals           int      `json:"decimals"`
	IconURL            string   `json:"icon_url"`
	HolderCount        int      `json:"holder_count"`
	TotalSupply        int64    `json:"total_supply"`
	MaxSupply          int64    `json:"max_supply"`
	IsFreezable        bool     `json:"is_freezable"`
	Description        *string  `json:"description"`
	WebsiteURL         *string  `json:"website_url"`
	TwitterURL         *string  `json:"twitter_url"`
	TelegramURL        *string  `json:"telegram_url"`
	CreatorPubkey      *string  `json:"creator_pubkey"`
	TokenCreatedAt     string   `json:"token_created_at"`
	TokenUpdatedAt     string   `json:"token_updated_at"`
	AggUpdatedAt       string   `json:"agg_updated_at"`
	Network            string   `json:"network"`
	AggVolume24hUsd    float64  `json:"agg_volume_24h_usd"`
	AggPriceChange24h  float64  `json:"agg_price_change_24h"`
	AggPriceUsd        float64  `json:"agg_price_usd"`
	AggPriceBtc        float64  `json:"agg_price_btc"`
	AggVolumeBtc       float64  `json:"agg_volume_btc"`
	AggMarketcapUsd    float64  `json:"agg_marketcap_usd"`
	AggTvlUsd          float64  `json:"agg_tvl_usd"`
	AthPriceUsd        float64  `json:"ath_price_usd"`
	AthMarketcapUsd    float64  `json:"ath_marketcap_usd"`
	HoldersUpdatedAt   string   `json:"holders_updated_at"`
	Top10HoldersPct    float64  `json:"top_10_holders_pct"`
	DevHoldingPct      *float64 `json:"dev_holding_pct"`
	AggPriceConfidence float64  `json:"agg_price_confidence"`
	Verified           bool     `json:"verified"`
}

// LuminexPoolExtra - extra pool data.
type LuminexPoolExtra struct {
	BondingCurveProgress float64 `json:"bondingCurveProgress"`
	Category             string  `json:"category"`
	MarketCapUsd         float64 `json:"marketCapUsd"`
	PoolTvlUsd           float64 `json:"poolTvlUsd"`
	Volume24hUsd         float64 `json:"volume24hUsd"`
	BundledPercentage    float64 `json:"bundledPercentage"`
}

// GetFullPoolData loads full pool data from Luminex.
func GetFullPoolData(poolLpPublicKey string) (*LuminexFullPoolResponse, error) {
	if poolLpPublicKey == "" {
		return nil, fmt.Errorf("poolLpPublicKey is required")
	}

	url := fmt.Sprintf("%s/%s", luminex.LuminexPoolAPIBaseURL, poolLpPublicKey)

	client := &http.Client{Timeout: 10 * time.Second}
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

	var poolResp LuminexFullPoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&poolResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}

	return &poolResp, nil
}

// CheckHotTokenConditionsFromSwaps checks hot token conditions for a pool using already fetched swaps.
func CheckHotTokenConditionsFromSwaps(swaps []flashnet.Swap, poolLpPublicKey string, swapsCount int, minAddresses int) (bool, int) {
	if poolLpPublicKey == "" {
		return false, 0
	}

	uniqueAddresses := make(map[string]bool)
	swapsForPool := 0

	for _, swap := range swaps {
		if swap.PoolLpPublicKey == poolLpPublicKey {
			swapsForPool++
			if swap.SwapperPublicKey != "" {
				uniqueAddresses[swap.SwapperPublicKey] = true
			}
			if swapsForPool >= swapsCount {
				break
			}
		}
	}

	uniqueCount := len(uniqueAddresses)
	if swapsForPool >= swapsCount && uniqueCount >= minAddresses {
		return true, uniqueCount
	}
	return false, uniqueCount
}

// CheckHotTokenConditions is legacy (kept for parity); prefer CheckHotTokenConditionsFromSwaps.
func CheckHotTokenConditions(client *flashnet.Client, poolLpPublicKey string, swapsCount int, minAddresses int) (bool, int, error) {
	if client == nil {
		return false, 0, fmt.Errorf("client is nil")
	}
	if poolLpPublicKey == "" {
		return false, 0, fmt.Errorf("poolLpPublicKey is required")
	}

	ctx := context.Background()
	limit := swapsCount * 10
	if limit > 1000 {
		limit = 1000
	}
	options := flashnet.GetSwapsOptions{Limit: &limit}

	swapsResp, err := client.GetSwaps(ctx, options)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get swaps: %w", err)
	}
	if swapsResp == nil || len(swapsResp.Swaps) == 0 {
		log.LogDebug("No swaps found", zap.String("poolLpPublicKey", poolLpPublicKey))
		return false, 0, nil
	}

	isHot, uniqueCount := CheckHotTokenConditionsFromSwaps(swapsResp.Swaps, poolLpPublicKey, swapsCount, minAddresses)
	log.LogDebug("Checked hot token conditions",
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("swapsCount", swapsCount),
		zap.Int("uniqueAddresses", uniqueCount),
		zap.Int("minAddresses", minAddresses),
		zap.Bool("isHot", isHot))

	return isHot, uniqueCount, nil
}

// GetAllUniquePoolsFromSwaps fetches recent swaps once and returns unique pool IDs + swaps payload.
func GetAllUniquePoolsFromSwaps(client *flashnet.Client, minSwapsToCheck int) ([]string, []flashnet.Swap, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client is nil")
	}

	ctx := context.Background()
	limit := 1000
	if minSwapsToCheck*10 > limit {
		limit = 1000
	} else {
		limit = minSwapsToCheck * 100
		if limit > 1000 {
			limit = 1000
		}
	}

	options := flashnet.GetSwapsOptions{Limit: &limit}
	swapsResp, err := client.GetSwaps(ctx, options)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get swaps: %w", err)
	}

	if swapsResp == nil || len(swapsResp.Swaps) == 0 {
		log.LogDebug("No swaps found for getting unique pools")
		return []string{}, []flashnet.Swap{}, nil
	}

	uniquePools := make(map[string]bool)
	for _, swap := range swapsResp.Swaps {
		if swap.PoolLpPublicKey != "" {
			uniquePools[swap.PoolLpPublicKey] = true
		}
	}

	result := make([]string, 0, len(uniquePools))
	for poolLpPublicKey := range uniquePools {
		result = append(result, poolLpPublicKey)
	}

	log.LogDebug("Extracted unique pools from swaps",
		zap.Int("totalSwaps", len(swapsResp.Swaps)),
		zap.Int("uniquePools", len(result)))

	return result, swapsResp.Swaps, nil
}



