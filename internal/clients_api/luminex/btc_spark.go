package luminex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// LuminexSparkAddressAPIBaseURL is Luminex API base for Spark addresses.
	LuminexSparkAddressAPIBaseURL = "https://api.luminex.io/spark/address"
	// SparkPublicKey is Spark address public key used for BTC reserve snapshots.
	SparkPublicKey = "023e33e2920326f64ea31058d44777442d97d7d5cbfcf54e3060bc1695e5261c93"
)

// BTCSparkAddressResponse is the subset of Luminex response used for BTC reserve.
type BTCSparkAddressResponse struct {
	SparkAddress string `json:"sparkAddress"`
	PublicKey    string `json:"publicKey"`
	Balance      struct {
		BtcSoftBalanceSats int64 `json:"btcSoftBalanceSats"`
	} `json:"balance"`
}

// GetBTCSparkReserve fetches BTC reserve for Spark public key from Luminex API.
// Returns reserve in BTC (rounded to 2 decimals like legacy logic).
func GetBTCSparkReserve() (float64, error) {
	url := fmt.Sprintf("%s/%s", LuminexSparkAddressAPIBaseURL, SparkPublicKey)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch from Luminex Spark Address API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("luminex Spark Address API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var addressResp BTCSparkAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addressResp); err != nil {
		return 0, fmt.Errorf("failed to decode Luminex Spark Address API response: %w", err)
	}

	// sats -> BTC and round to 2 decimals (legacy behavior)
	btcReserve := float64(addressResp.Balance.BtcSoftBalanceSats) / 1e8
	btcReserve = float64(int(btcReserve*100+0.5)) / 100
	return btcReserve, nil
}

