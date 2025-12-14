package system_works

// Package flashnet contains for Flashnet AMM API
// data API

// NativeTokenAddress - address token (BTC) in Flashnet
// for (/)
const NativeTokenAddress = "020202020202020202020202020202020202020202020202020202020202020202"

// SwapType swap
type SwapType string

const (
	SwapTypeBuy  SwapType = "BUY"  // token token (BTC)
	SwapTypeSell SwapType = "SELL" // token token (BTC)
	SwapTypeSwap SwapType = "SWAP" // token)
)

// Swap swap tokens)
// struct is in Go
// string, API
type Swap struct {
	AmountIn           string `json:"amountIn"`        // count tokens on
	AmountOut          string `json:"amountOut"`       // count tokens on
	AssetInAddress     string `json:"assetInAddress"`  // address token,
	AssetOutAddress    string `json:"assetOutAddress"` // address token, get
	CreatedAt          string `json:"createdAt"`       // time
	FeePaid            string `json:"feePaid"`
	ID                 string `json:"id"`
	InboundTransferID  string `json:"inboundTransferId"`  // ID
	OutboundTransferID string `json:"outboundTransferId"` // ID
	PoolAssetAAddress  string `json:"poolAssetAAddress"`  // address token in
	PoolAssetBAddress  string `json:"poolAssetBAddress"`  // address token in
	PoolLpPublicKey    string `json:"poolLpPublicKey"`    // pool
	PoolType           string `json:"poolType"`           // pool (CONSTANT_PRODUCT or SINGLE_SIDED)
	Price              string `json:"price"`
	SwapperPublicKey   string `json:"swapperPublicKey"` // swap
	Timestamp          string `json:"timestamp"`        // CreatedAt)
}

// SwapsResponse API swaps
// swaps and count
// for GET /swaps
type SwapsResponse struct {
	Swaps      []Swap `json:"swaps"`      // swaps ([]Swap Swap)
	TotalCount int    `json:"totalCount"` // count swaps (int -
}

// UserSwapsResponse API swaps
// for GET /swaps/user/{userPublicKey}
type UserSwapsResponse struct {
	Swaps      []Swap `json:"swaps"`      // swaps
	TotalCount int    `json:"totalCount"` // count swaps
}

// GetSwapsOptions for and swaps
// (options), in GetSwaps
// nil (null),
// Using pointers (*int, *string), we can distinguish "not specified" (nil) and "specified as 0" (0)
// By default all parameters = null (nil), which means using API default values
type GetSwapsOptions struct {
	Limit        *int    // count swaps in (1-1000, by default: null = value API by default)
	Offset       *int    // count swaps, (for by default: null)
	PoolType     *string // pool for ("CONSTANT_PRODUCT" or "SINGLE_SIDED", by default: null)
	AssetAddress *string // address token for by default: null)
	StartTime    *string // time in RFC3339 "2025-01-01T00:00:00Z", by default: null)
	EndTime      *string // time in RFC3339 (by default: null)
}

type GetUserSwapsOptions struct {
	PoolLpPubkey    string // pool for
	AssetInAddress  string // address token for
	AssetOutAddress string // address token for
	MinAmountIn     int    // count tokens
	MaxAmountIn     int    // count tokens
	StartTime       string // time in RFC3339
	EndTime         string // time in RFC3339
	Sort            string // (timestampDesc, timestampAsc, amountInDesc and ..)
	Limit           int    // count swaps (1-100, by default: 20)
	Offset          int    // count swaps for (by default: 0)
}

// GetSwapType swap on tokens
// - SwapTypeBuy: if assetInAddress == NativeTokenAddress token BTC)
// - SwapTypeSell: if assetOutAddress == NativeTokenAddress token BTC)
// - SwapTypeSwap: if token
func (s *Swap) GetSwapType() SwapType {
	isNativeIn := s.AssetInAddress == NativeTokenAddress
	isNativeOut := s.AssetOutAddress == NativeTokenAddress

	if isNativeIn && !isNativeOut {
		return SwapTypeBuy // BTC, get token
	} else if !isNativeIn && isNativeOut {
		return SwapTypeSell // token, get BTC
	}
	return SwapTypeSwap
}

// IsBuy swap token token (BTC)
func (s *Swap) IsBuy() bool {
	return s.GetSwapType() == SwapTypeBuy
}

// IsSell swap token token (BTC)
func (s *Swap) IsSell() bool {
	return s.GetSwapType() == SwapTypeSell
}
