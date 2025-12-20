package flashnet

// Package flashnet contains client for Flashnet AMM API
// This file contains base HTTP client - handles all HTTP requests to API
// Acts as transport layer - doesn't know business logic, just sends requests and receives responses

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"spark-wallet/internal/infra/log"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	// AMMMainnetAPI - URL for main Flashnet API (production)
	AMMMainnetAPI = "https://api.flashnet.xyz/v1"
	// AMMTestnetAPI - URL for test Flashnet API (for testing)
	AMMTestnetAPI = "https://api.makebitcoingreatagain.dev/v1"
)

func GenerateRequestID() string { return log.GenerateRequestID() }

func LogRequest(requestID, method, endpoint string, fields ...zap.Field) {
	log.LogRequest(requestID, method, endpoint, fields...)
}

func LogResponse(requestID string, statusCode int, durationMs int64, fields ...zap.Field) {
	log.LogResponse(requestID, statusCode, durationMs, fields...)
}

func LogDebug(message string, fields ...zap.Field)   { log.LogDebug(message, fields...) }
func LogError(message string, fields ...zap.Field)   { log.LogError(message, fields...) }
func LogInfo(message string, fields ...zap.Field)    { log.LogInfo(message, fields...) }
func LogWarn(message string, fields ...zap.Field)    { log.LogWarn(message, fields...) }
func LogSuccess(message string, fields ...zap.Field) { log.LogSuccess(message, fields...) }

// Client is a struct containing API client data
// Stores everything needed for API work: base URL, HTTP client and token
type Client struct {
	baseURL         string                    // Base API URL (mainnet or testnet)
	httpClient      *http.Client              // HTTP client for requests
	jwtToken        string                    // JWT token for authorized requests (can be empty if not authorized)
	rateLimiter     *rate.Limiter             // Rate limiter for request frequency limiting
	circuitBreaker  *gobreaker.CircuitBreaker // Circuit breaker for error avalanche protection
	maxResponseSize int64                     // Maximum response size in bytes
}

// NewAMMClient is a constructor function
// Creates and returns new Client object ready to use
// network - network name string ("mainnet" or "testnet")
func NewAMMClient(network string) *Client {
	// Default to mainnet (main network)
	baseURL := AMMMainnetAPI
	// If testnet specified, use test URL
	if network == "testnet" {
		baseURL = AMMTestnetAPI
	}

	// Create rate limiter: 10 requests per second, burst up to 20
	rateLimiter := rate.NewLimiter(rate.Limit(10), 20)

	// Create circuit breaker for error avalanche protection
	circuitBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "FlashnetAPI",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})

	return &Client{
		baseURL:         baseURL,
		rateLimiter:     rateLimiter,
		circuitBreaker:  circuitBreaker,
		maxResponseSize: 10 * 1024 * 1024, // 10MB default
		httpClient: &http.Client{
			// Timeout - maximum wait time for server response
			// 30 * time.Second means 30 seconds
			// If server doesn't respond within 30 seconds, request will be cancelled
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Enable keep-alive for better Cloudflare compatibility
				DisableKeepAlives: false,
				MaxIdleConns:      10,
				IdleConnTimeout:   90 * time.Second,
			},
		},
	}
}

// SetJWT is a method of Client struct
// Methods in Go are declared as functions with receiver
// (c *Client) Client
// *Client on Client
// token - JWT for
func (c *Client) SetJWT(token string) {
	// Save JWT token in
	// token in Authorization
	c.jwtToken = token
}

// GetJWT JWT token
func (c *Client) GetJWT() string {
	return c.jwtToken
}

// MakeRequest HTTP API rate limiting and circuit breaker
// ctx - for and
// method - HTTP (GET, POST, PUT, DELETE and ..)
// endpoint - API "/swaps" or "/auth/challenge")
// body - nil for GET
// []byte (data and error (error, if
func (c *Client) MakeRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	// Generate request ID for
	requestID := GenerateRequestID()
	startTime := time.Now()

	// Check
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	}

	// rate limiter 429
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait failed: %w", err)
		}
	}

	// circuit breaker
	var respBody []byte
	var err error

	if c.circuitBreaker != nil {
		_, err = c.circuitBreaker.Execute(func() (interface{}, error) {
			body, err := c.makeRequestWithContext(ctx, requestID, method, endpoint, body, startTime)
			if err != nil {
				return nil, err
			}
			respBody = body
			return body, nil
		})
		if err != nil {
			LogError("Circuit breaker rejected request", zap.String("request_id", requestID), zap.String("endpoint", endpoint), zap.Error(err))
			return nil, err
		}
	} else {
		respBody, err = c.makeRequestWithContext(ctx, requestID, method, endpoint, body, startTime)
		if err != nil {
			return nil, err
		}
	}

	duration := time.Since(startTime).Milliseconds()

	// log in file LogResponse)
	// SUCCESS in LogSuccess
	LogResponse(requestID, 200, duration, zap.String("endpoint", endpoint))

	return respBody, nil
}

// makeRequestWithContext HTTP
func (c *Client) makeRequestWithContext(ctx context.Context, requestID, method, endpoint string, body interface{}, startTime time.Time) ([]byte, error) {
	var reqBody io.Reader

	// Check,
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// Create
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setNormalizedHeaders(req, c.jwtToken)

	LogRequest(requestID, method, endpoint, zap.String("url", req.URL.String()))

	// HTTP
	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(startTime).Milliseconds()
		LogResponse(requestID, 0, duration, zap.String("endpoint", endpoint), zap.Error(err))
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, c.maxResponseSize)

	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		duration := time.Since(startTime).Milliseconds()
		LogResponse(requestID, resp.StatusCode, duration, zap.String("endpoint", endpoint), zap.Error(err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()

	// Check
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && !strings.Contains(contentType, "application/json") {
			LogResponse(requestID, resp.StatusCode, duration, zap.String("endpoint", endpoint), zap.String("error", "blocked by Cloudflare or invalid response"))
			return nil, fmt.Errorf("API error (%d): blocked by Cloudflare or invalid response", resp.StatusCode)
		}
		LogResponse(requestID, resp.StatusCode, duration, zap.String("endpoint", endpoint), zap.String("error", "API error response received"))
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	LogResponse(requestID, resp.StatusCode, duration, zap.String("endpoint", endpoint), zap.String("status", "success"))

	return respBody, nil
}

// setNormalizedHeaders HTTP
func setNormalizedHeaders(req *http.Request, jwtToken string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Add JWT token in Authorization, if
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
}

// GetSwaps swaps from AMM (Automated Market Maker)
// ctx - for and
// options - and nil = null)
// If nil, in (API value by default)
// on SwapsResponse and error
func (c *Client) GetSwaps(ctx context.Context, options GetSwapsOptions) (*SwapsResponse, error) {
	params := url.Values{}

	if options.Limit != nil {
		params.Set("limit", strconv.Itoa(*options.Limit))
	}
	if options.Offset != nil {
		params.Set("offset", strconv.Itoa(*options.Offset))
	}
	if options.PoolType != nil && *options.PoolType != "" {
		params.Set("pool_type", *options.PoolType)
	}
	if options.AssetAddress != nil && *options.AssetAddress != "" {
		params.Set("asset_address", *options.AssetAddress)
	}
	if options.StartTime != nil && *options.StartTime != "" {
		params.Set("start_time", *options.StartTime)
	}
	if options.EndTime != nil && *options.EndTime != "" {
		params.Set("end_time", *options.EndTime)
	}

	endpoint := "/swaps"
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	respBody, err := c.MakeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get swaps: %w", err)
	}

	var swapsResp SwapsResponse
	if err := json.Unmarshal(respBody, &swapsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal swaps response: %w", err)
	}

	return &swapsResp, nil
}

// GetUserSwaps swaps for by
// ctx - for and
// userPublicKey - (hex
// on UserSwapsResponse and error
func (c *Client) GetUserSwaps(ctx context.Context, userPublicKey string, options GetUserSwapsOptions) (*UserSwapsResponse, error) {
	params := url.Values{}

	if options.PoolLpPubkey != "" {
		params.Set("poolLpPubkey", options.PoolLpPubkey)
	}
	if options.AssetInAddress != "" {
		params.Set("assetInAddress", options.AssetInAddress)
	}
	if options.AssetOutAddress != "" {
		params.Set("assetOutAddress", options.AssetOutAddress)
	}
	if options.MinAmountIn > 0 {
		params.Set("minAmountIn", strconv.Itoa(options.MinAmountIn))
	}
	if options.MaxAmountIn > 0 {
		params.Set("maxAmountIn", strconv.Itoa(options.MaxAmountIn))
	}
	if options.StartTime != "" {
		params.Set("startTime", options.StartTime)
	}
	if options.EndTime != "" {
		params.Set("endTime", options.EndTime)
	}
	if options.Sort != "" {
		params.Set("sort", options.Sort)
	}
	if options.Limit > 0 {
		params.Set("limit", strconv.Itoa(options.Limit))
	}
	if options.Offset > 0 {
		params.Set("offset", strconv.Itoa(options.Offset))
	}

	endpoint := fmt.Sprintf("/swaps/user/%s", userPublicKey)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	respBody, err := c.MakeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user swaps: %w", err)
	}

	var userSwapsResp UserSwapsResponse
	if err := json.Unmarshal(respBody, &userSwapsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user swaps response: %w", err)
	}

	// log swaps for (if
	if len(userSwapsResp.Swaps) > 0 {
		for i := 0; i < len(userSwapsResp.Swaps) && i < 2; i++ {
			swap := userSwapsResp.Swaps[i]
			LogDebug("Raw swap from GetUserSwaps",
				zap.Int("index", i),
				zap.String("swapID", swap.ID),
				zap.String("timestamp", swap.Timestamp),
				zap.String("poolLpPublicKey", swap.PoolLpPublicKey),
				zap.Bool("hasTimestamp", swap.Timestamp != ""))
		}
	}

	return &userSwapsResp, nil
}

// ============================================
// for (Challenge/Verify)
// ============================================

// ChallengeRequest is on challenge
type ChallengeRequest struct {
	PublicKey string `json:"publicKey"` // in hex
}

// ChallengeResponse - API challenge
type ChallengeResponse struct {
	Challenge       string `json:"challenge"`       // Challenge in (hex
	ChallengeString string `json:"challengeString"` // Challenge in for
	RequestID       string `json:"requestId"`       // ID for
}

type VerifyRequest struct {
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"` // in DER hex
}

// VerifyResponse -
type VerifyResponse struct {
	AccessToken string `json:"accessToken"` // JWT token for
}

// ChallengeFile - for challenge in file
type ChallengeFile struct {
	PublicKey       string `json:"publicKey"`
	ChallengeString string `json:"challengeString"` // Challenge for
	RequestID       string `json:"requestId"`       // ID (for signature.json)
}

// publicKey and requestId from challenge.json
type SignatureFile struct {
	PublicKey string `json:"publicKey"` // from challenge.json
	Signature string `json:"signature"` // challengeString
	RequestID string `json:"requestId"` // from challenge.json
}

// TokenFile - for JWT token
type TokenFile struct {
	AccessToken string `json:"accessToken"` // JWT token for
	ExpiresAt   int64  `json:"expiresAt"`   // time token (Unix timestamp)
	PublicKey   string `json:"publicKey"`   // for token
}

// JWTToken - for JWT token for time
type JWTToken struct {
	ExpiresAt int64 `json:"exp"` // time token (Unix timestamp in
	IssuedAt  int64 `json:"iat"` // time token (Unix timestamp in
}

// ============================================
// ============================================

// SaveChallengeToFile challenge in file for
func SaveChallengeToFile(dataDir string, challengeResp *ChallengeResponse, publicKey string) (string, error) {
	challengeFile := ChallengeFile{
		PublicKey:       publicKey,
		ChallengeString: challengeResp.ChallengeString,
		RequestID:       challengeResp.RequestID,
	}

	jsonData, err := json.MarshalIndent(challengeFile, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal challenge file: %w", err)
	}

	filename := filepath.Join(dataDir, "challenge.json")
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save challenge file: %w", err)
	}

	return filename, nil
}

// updateSignChallengeFile challengeString in sign-challenge.mjs
func updateSignChallengeFile(dataDir string, challengeString string) {
	// sign-challenge.mjs
	// dataDir "data_in" or "cmd/bot/data", use
	signChallengePaths := []string{
		filepath.Join("spark-cli", "sign-challenge.mjs"), // If start from
		filepath.Join(".", "spark-cli", "sign-challenge.mjs"),
		filepath.Join(dataDir, "..", "..", "spark-cli", "sign-challenge.mjs"), // If dataDir = cmd/bot/data
		filepath.Join(dataDir, "..", "spark-cli", "sign-challenge.mjs"),       // If dataDir = data_in
	}

	var signChallengePath string
	for _, path := range signChallengePaths {
		if _, err := os.Stat(path); err == nil {
			signChallengePath = path
			break
		}
	}

	if signChallengePath == "" {
		LogDebug("Could not find sign-challenge.mjs", zap.Strings("tried_paths", signChallengePaths))
		return
	}

	data, err := os.ReadFile(signChallengePath)
	if err != nil {
		LogDebug("Could not read sign-challenge.mjs", zap.String("path", signChallengePath), zap.Error(err))
		return
	}

	fileContent := string(data)

	// const challengeString = process.argv[2] || "FLASHNET_AUTH_CHALLENGE_V1:...";
	pattern1 := regexp.MustCompile(`(const challengeString = process\.argv\[2\] \|\| )"[^"]*"`)
	if pattern1.MatchString(fileContent) {
		newContent := pattern1.ReplaceAllString(fileContent, fmt.Sprintf(`$1"%s"`, challengeString))
		if err := os.WriteFile(signChallengePath, []byte(newContent), 0644); err != nil {
			LogWarn("Failed to update sign-challenge.mjs", zap.Error(err))
			return
		}
		LogInfo("Updated challengeString in sign-challenge.mjs", zap.String("file", signChallengePath))
		return
	}

	// If process.argv[2])
	pattern2 := regexp.MustCompile(`(const challengeString = )"[^"]*"`)
	if pattern2.MatchString(fileContent) {
		newContent := pattern2.ReplaceAllString(fileContent, fmt.Sprintf(`$1"%s"`, challengeString))
		if err := os.WriteFile(signChallengePath, []byte(newContent), 0644); err != nil {
			LogWarn("Failed to update sign-challenge.mjs", zap.Error(err))
			return
		}
		LogInfo("Updated challengeString in sign-challenge.mjs", zap.String("file", signChallengePath))
		return
	}

	LogWarn("Could not find challengeString pattern in sign-challenge.mjs")
}

// publicKey and requestId from challenge.json if
func LoadSignatureFromFile(dataDir string) (*SignatureFile, error) {
	filename := filepath.Join(dataDir, "signature.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature file: %w", err)
	}

	var sigFile SignatureFile
	if err := json.Unmarshal(data, &sigFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signature file: %w", err)
	}

	// publicKey and requestId from challenge.json if
	challengeFile, err := LoadChallengeFromFile(dataDir)
	if err == nil {
		// update publicKey and requestId from challenge.json
		sigFile.PublicKey = challengeFile.PublicKey
		sigFile.RequestID = challengeFile.RequestID
		jsonData, _ := json.MarshalIndent(sigFile, "", "  ")
		os.WriteFile(filename, jsonData, 0644)
	}

	return &sigFile, nil
}

// LoadChallengeFromFile challenge from file
func LoadChallengeFromFile(dataDir string) (*ChallengeFile, error) {
	filename := filepath.Join(dataDir, "challenge.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read challenge file: %w", err)
	}

	var challengeFile ChallengeFile
	if err := json.Unmarshal(data, &challengeFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal challenge file: %w", err)
	}

	return &challengeFile, nil
}

// SaveTokenToFile JWT token in file
func SaveTokenToFile(dataDir string, accessToken string, publicKey string, expiresAt int64) (string, error) {
	tokenFile := TokenFile{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
		PublicKey:   publicKey,
	}

	jsonData, err := json.MarshalIndent(tokenFile, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal token file: %w", err)
	}

	filename := filepath.Join(dataDir, "token.json")
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save token file: %w", err)
	}

	return filename, nil
}

// LoadTokenFromFile token from file
func LoadTokenFromFile(dataDir string) (*TokenFile, error) {
	filename := filepath.Join(dataDir, "token.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var tokenFile TokenFile
	if err := json.Unmarshal(data, &tokenFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token file: %w", err)
	}

	return &tokenFile, nil
}

// GetTokenExpirationTime time from JWT token
func GetTokenExpirationTime(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid JWT token format")
	}

	payload := parts[1]
	if len(payload)%4 != 0 {
		payload += strings.Repeat("=", 4-len(payload)%4)
	}

	payloadBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var jwtToken JWTToken
	if err := json.Unmarshal(payloadBytes, &jwtToken); err != nil {
		return 0, fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}

	if jwtToken.ExpiresAt == 0 {
		return 0, fmt.Errorf("JWT token does not contain expiration time")
	}

	return jwtToken.ExpiresAt, nil
}

// ============================================
// API
// ============================================

// GetChallengeAndSave challenge Flashnet API and in file
// dataDir - for "cmd/bot/data")
// publicKey - for challenge
// challenge.json and error
func (c *Client) GetChallengeAndSave(ctx context.Context, dataDir string, publicKey string) (string, error) {
	startTime := time.Now()
	req := ChallengeRequest{
		PublicKey: publicKey,
	}

	respBody, err := c.MakeRequest(ctx, "POST", "/auth/challenge", req)
	duration := time.Since(startTime).Milliseconds()
	if err != nil {
		return "", fmt.Errorf("failed to get challenge: %w", err)
	}

	var challengeResp ChallengeResponse
	if err := json.Unmarshal(respBody, &challengeResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal challenge response: %w", err)
	}

	// save challenge in file
	filename, err := SaveChallengeToFile(dataDir, &challengeResp, publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to save challenge file: %w", err)
	}

	// signature.json challenge
	// Update publicKey and requestId, (if
	sigFile := SignatureFile{
		PublicKey: publicKey,
		Signature: "",
		RequestID: challengeResp.RequestID,
	}

	signatureFilename := filepath.Join(dataDir, "signature.json")
	if data, err := os.ReadFile(signatureFilename); err == nil {
		var existingSigFile SignatureFile
		if json.Unmarshal(data, &existingSigFile) == nil && existingSigFile.Signature != "" {
			// Save if
			sigFile.Signature = existingSigFile.Signature
		}
	}

	// Save signature.json
	jsonData, err := json.MarshalIndent(sigFile, "", "  ")
	if err == nil {
		os.WriteFile(signatureFilename, jsonData, 0644)
	}

	// update challengeString in sign-challenge.mjs
	updateSignChallengeFile(dataDir, challengeResp.ChallengeString)

	LogSuccess("Challenge received and saved", zap.String("file", filename), zap.Int64("duration_ms", duration))

	return filename, nil
}

// VerifySignatureAndSave and JWT token, in file
// publicKey -
// token.json and error
func (c *Client) VerifySignatureAndSave(ctx context.Context, dataDir string, publicKey string, signature string) (string, error) {
	startTime := time.Now()
	req := VerifyRequest{
		PublicKey: publicKey,
		Signature: signature,
	}

	respBody, err := c.MakeRequest(ctx, "POST", "/auth/verify", req)
	duration := time.Since(startTime).Milliseconds()
	if err != nil {
		return "", fmt.Errorf("failed to verify signature: %w", err)
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal verify response: %w", err)
	}

	// time from token
	expiresAt, err := GetTokenExpirationTime(verifyResp.AccessToken)
	if err != nil {
		// If time, use 24 by default
		expiresAt = time.Now().Add(24 * time.Hour).Unix()
	}

	// save token in file
	filename, err := SaveTokenToFile(dataDir, verifyResp.AccessToken, publicKey, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to save token file: %w", err)
	}

	// Save token in for
	c.SetJWT(verifyResp.AccessToken)

	LogSuccess("JWT token obtained and saved", zap.String("file", filename), zap.Int64("duration_ms", duration))

	return filename, nil
}

// GetChallenge challenge Flashnet API for
// file - GetChallengeAndSave for
func (c *Client) GetChallenge(ctx context.Context, publicKey string) (*ChallengeResponse, error) {
	req := ChallengeRequest{
		PublicKey: publicKey,
	}

	respBody, err := c.MakeRequest(ctx, "POST", "/auth/challenge", req)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	var challengeResp ChallengeResponse
	if err := json.Unmarshal(respBody, &challengeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal challenge response: %w", err)
	}

	return &challengeResp, nil
}

// VerifySignature and JWT token for
// file - VerifySignatureAndSave for
func (c *Client) VerifySignature(ctx context.Context, publicKey, signature string) (*VerifyResponse, error) {
	req := VerifyRequest{
		PublicKey: publicKey,
		Signature: signature,
	}

	respBody, err := c.MakeRequest(ctx, "POST", "/auth/verify", req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal verify response: %w", err)
	}

	c.SetJWT(verifyResp.AccessToken)

	return &verifyResp, nil
}

// IsAlreadySignedInError error "already signed in"
func IsAlreadySignedInError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "already signed in") ||
		strings.Contains(errStr, "FSAG-4102") ||
		strings.Contains(errStr, "User already signed in")
}
