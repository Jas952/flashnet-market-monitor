package luminex

// HTTP client for working with Luminex API
// Sends GET requests to Luminex API with retry mechanism
// Sets Cloudflare-friendly headers to bypass protection
// Uses common retry module for error handling

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"spark-wallet/internal/infra/retry"
)

var luminexHTTPTimeout = 10 * time.Second
var luminexRetry = retry.Options{
	MaxRetries: 3,
	BaseDelay:  300 * time.Millisecond,
	MaxDelay:   5 * time.Second,
	Backoff:    2.0,
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: luminexHTTPTimeout}
}

func setCloudflareHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://luminex.io/")
	req.Header.Set("Origin", "https://luminex.io")
	req.Header.Set("Connection", "keep-alive")
}

func doGET(ctx context.Context, url string) ([]byte, error) {
	client := newHTTPClient()

	var respBody []byte
	err := retry.Do(ctx, luminexRetry, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		setCloudflareHeaders(req)

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		respBody = body

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &retry.HTTPError{
				StatusCode: resp.StatusCode,
				Body:       body,
				RetryAfter: retry.ParseRetryAfter(resp.Header.Get("Retry-After")),
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("luminex GET failed: %w", err)
	}
	return respBody, nil
}

func DoGET(ctx context.Context, url string) ([]byte, error) {
	return doGET(ctx, url)
}
