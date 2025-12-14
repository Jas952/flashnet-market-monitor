package system_works

// Package system_works contains for tokens from Luminex API

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

const (
	// LuminexHoldersAPIBaseURL - URL API Luminex for
	LuminexHoldersAPIBaseURL = "https://api.luminex.io/spark/holders"
)

// HoldersResponse - API Luminex for
type HoldersResponse struct {
	Data []HolderData `json:"data"`
	Meta HoldersMeta  `json:"meta"`
}

// HoldersMeta - API for
type HoldersMeta struct {
	TotalItems int `json:"totalItems"` // count
	Limit      int `json:"limit"`      // on
	Offset     int `json:"offset"`     // (offset)
}

// HolderData - from API
type HolderData struct {
	TokenIdentifier string `json:"token_identifier"`
	Address         string `json:"address"` // address wallet
	Network         string `json:"network"`
	Pubkey          string `json:"pubkey"`
	Balance         string `json:"balance"` // balance tokens
	LastActivity    string `json:"last_activity"`
	TransferCount   int    `json:"transfer_count"`
	IsPool          bool   `json:"is_pool"`
}

// Holder - token for
type Holder struct {
	TokenIdentifier string    `json:"token_identifier"`
	Address         string    `json:"address"`
	Network         string    `json:"network"`
	Pubkey          string    `json:"pubkey"`
	Balance         string    `json:"balance"`        // balance tokens for
	BalanceFloat    float64   `json:"-"`              // balance in float64 (for
	LastActivity    string    `json:"last_activity"`  // time
	TransferCount   int       `json:"transfer_count"` // count
	IsPool          bool      `json:"is_pool"`
	LastUpdated     time.Time `json:"-"` // time in JSON)
}

func GetHolders(tokenIdentifier string, limit int, offset int) (*HoldersResponse, error) {
	if tokenIdentifier == "" {
		return nil, fmt.Errorf("token identifier is empty")
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	url := fmt.Sprintf("%s?tokenIdentifier=%s&limit=%d&offset=%d", LuminexHoldersAPIBaseURL, tokenIdentifier, limit, offset)

	client := &http.Client{
		Timeout: 30 * time.Second,
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

	var holdersResp HoldersResponse
	if err := json.NewDecoder(resp.Body).Decode(&holdersResp); err != nil {
		return nil, fmt.Errorf("failed to decode Luminex API response: %w", err)
	}

	return &holdersResp, nil
}

// LoadTokenIdentifiers tokens from JSON file
func LoadTokenIdentifiers(filename string) (map[string]string, error) {
	type TokenIDsFile struct {
		IDTokens map[string]string `json:"id_tokens"`
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read token identifiers file: %w", err)
	}

	var file TokenIDsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse token identifiers JSON: %w", err)
	}

	return file.IDTokens, nil
}

// GetAllHolders token,
func GetAllHolders(tokenIdentifier string) (*HoldersResponse, error) {
	if tokenIdentifier == "" {
		return nil, fmt.Errorf("token identifier is empty")
	}

	firstPage, err := GetHolders(tokenIdentifier, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get first page of holders: %w", err)
	}

	totalItems := firstPage.Meta.TotalItems
	if totalItems == 0 {
		// If use count from data
		totalItems = len(firstPage.Data)
	}

	LogInfo("Fetching all holders", zap.String("tokenIdentifier", tokenIdentifier), zap.Int("totalItems", totalItems), zap.Int("firstPageCount", len(firstPage.Data)))

	if len(firstPage.Data) >= totalItems {
		LogInfo("All holders fit in first page", zap.Int("count", len(firstPage.Data)))
		return firstPage, nil
	}

	LogInfo("Need to fetch more pages", zap.Int("totalItems", totalItems), zap.Int("firstPageCount", len(firstPage.Data)))

	allHolders := make([]HolderData, 0, totalItems)
	allHolders = append(allHolders, firstPage.Data...)

	// from or use 100 by default
	limit := firstPage.Meta.Limit
	if limit == 0 {
		limit = 100
	}

	// Get
	pageNum := 2
	for offset := limit; offset < totalItems; offset += limit {
		page, err := GetHolders(tokenIdentifier, limit, offset)
		if err != nil {
			LogWarn("Failed to get holders page", zap.Int("offset", offset), zap.Int("page", pageNum), zap.Error(err))
			pageNum++
			continue
		}

		allHolders = append(allHolders, page.Data...)
		LogInfo("Loaded holders page", zap.Int("page", pageNum), zap.Int("offset", offset), zap.Int("count", len(page.Data)), zap.Int("totalSoFar", len(allHolders)))

		if len(page.Data) < limit {
			break
		}

		pageNum++
	}

	LogInfo("Finished fetching all holders",
		zap.String("tokenIdentifier", tokenIdentifier),
		zap.Int("totalFetched", len(allHolders)),
		zap.Int("expectedTotal", totalItems))

	return &HoldersResponse{
		Data: allHolders,
		Meta: HoldersMeta{
			TotalItems: totalItems, // Use original totalItems, len(allHolders)
			Limit:      limit,
			Offset:     0,
		},
	}, nil
}
