package system_works

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// GetFirstBuySwap returns date/time (MSK) of first buy-swap for user+pool
func GetFirstBuySwap(client *Client, userPubkey string, poolLpPublicKey string) (string, error) {
	if userPubkey == "" || poolLpPublicKey == "" {
		return "", fmt.Errorf("userPubkey and poolLpPublicKey are required")
	}

	if client == nil {
		LogDebug("Client is nil, cannot fetch first buy swap", zap.String("userPubkey", userPubkey), zap.String("poolLpPublicKey", poolLpPublicKey))
		return "", nil
	}

	ctx := context.Background()

	// /swaps/user
	options := GetUserSwapsOptions{
		PoolLpPubkey: poolLpPublicKey, // by query
		Limit:        1000,
		Sort:         "timestampAsc", // - buy swap
	}

	userSwapsResp, err := client.GetUserSwaps(ctx, userPubkey, options)
	if err != nil {
		LogDebug("Failed to fetch user swaps from Flashnet API",
			zap.String("userPubkey", userPubkey),
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.Error(err))
		return "", nil // return error,
	}

	if userSwapsResp == nil || len(userSwapsResp.Swaps) == 0 {
		LogDebug("No swaps found for user and pool",
			zap.String("userPubkey", userPubkey),
			zap.String("poolLpPublicKey", poolLpPublicKey))
		return "", nil
	}

	LogDebug("Found swaps for user and pool",
		zap.String("userPubkey", userPubkey),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalSwaps", len(userSwapsResp.Swaps)))

	if len(userSwapsResp.Swaps) > 0 {
		for i := 0; i < len(userSwapsResp.Swaps) && i < 3; i++ {
			swap := &userSwapsResp.Swaps[i]
			LogDebug("Sample swap from user endpoint",
				zap.Int("index", i),
				zap.String("swapID", swap.ID),
				zap.String("timestamp", swap.Timestamp),
				zap.String("poolLpPublicKey", swap.PoolLpPublicKey),
				zap.String("assetInAddress", swap.AssetInAddress),
				zap.String("assetOutAddress", swap.AssetOutAddress))
		}
	}

	var firstBuySwap *Swap
	var firstBuyTime time.Time

	for i := range userSwapsResp.Swaps {
		swap := &userSwapsResp.Swaps[i]

		if swap.PoolLpPublicKey != poolLpPublicKey {
			continue
		}

		// Use from swaps_types.go for buy swap
		isBuy := swap.AssetInAddress == NativeTokenAddress && swap.AssetOutAddress != NativeTokenAddress

		if isBuy {
			if swap.Timestamp == "" {
				LogDebug("Buy swap has empty Timestamp (skipping)",
					zap.String("swapID", swap.ID),
					zap.String("assetInAddress", swap.AssetInAddress),
					zap.String("assetOutAddress", swap.AssetOutAddress),
					zap.Int("index", i))
				continue
			}

			LogDebug("Found buy swap",
				zap.Int("index", i),
				zap.String("swapID", swap.ID),
				zap.String("timestamp", swap.Timestamp),
				zap.String("assetInAddress", swap.AssetInAddress),
				zap.String("assetOutAddress", swap.AssetOutAddress))

			timeStr := swap.Timestamp

			// Parse time from ISO 8601 (RFC3339)
			createdTime, err := time.Parse(time.RFC3339, timeStr)
			if err != nil {
				LogDebug("Failed to parse timestamp",
					zap.String("timestamp", timeStr),
					zap.String("swapID", swap.ID),
					zap.Error(err))
				continue
			}

			if firstBuySwap == nil || createdTime.Before(firstBuyTime) {
				firstBuySwap = swap
				firstBuyTime = createdTime
				LogDebug("Found earlier buy swap from user endpoint",
					zap.String("swapID", swap.ID),
					zap.String("timestamp", timeStr),
					zap.Time("parsedTime", createdTime),
					zap.Int("index", i))
			}
		}
	}

	if firstBuySwap != nil && firstBuySwap.Timestamp != "" {
		moscowLocation, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			LogDebug("Failed to load Moscow timezone, using UTC", zap.Error(err))
			moscowLocation = time.UTC
		}
		moscowTime := firstBuyTime.In(moscowLocation)
		formattedDate := moscowTime.Format("2006-01-02 15:04")

		LogDebug("First buy swap found via user endpoint and formatted",
			zap.String("userPubkey", userPubkey),
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.String("swapID", firstBuySwap.ID),
			zap.String("timestamp", firstBuySwap.Timestamp),
			zap.String("formattedDate", formattedDate),
			zap.Time("moscowTime", moscowTime),
			zap.Int("totalSwaps", len(userSwapsResp.Swaps)))
		return formattedDate, nil
	}

	LogDebug("No buy swap with timestamp found via /swaps/user, trying main /swaps endpoint",
		zap.String("userPubkey", userPubkey),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalSwapsFromUserEndpoint", len(userSwapsResp.Swaps)))

	var allSwapsFromMain []Swap
	limitMain := 1000
	offsetMain := 0

	for {
		optionsMain := GetSwapsOptions{
			Limit:  &limitMain,
			Offset: &offsetMain,
		}

		swapsResp, err := client.GetSwaps(ctx, optionsMain)
		if err != nil {
			LogDebug("Failed to fetch swaps from main endpoint",
				zap.String("userPubkey", userPubkey),
				zap.String("poolLpPublicKey", poolLpPublicKey),
				zap.Int("offset", offsetMain),
				zap.Error(err))
			break
		}

		if swapsResp == nil || len(swapsResp.Swaps) == 0 {
			break
		}

		for i := range swapsResp.Swaps {
			swap := &swapsResp.Swaps[i]
			if swap.PoolLpPublicKey == poolLpPublicKey && swap.SwapperPublicKey == userPubkey {
				allSwapsFromMain = append(allSwapsFromMain, *swap)
			}
		}

		if len(swapsResp.Swaps) < limitMain {
			break
		}

		offsetMain += limitMain

		if offsetMain >= 10000 {
			break
		}
	}

	LogDebug("Found swaps from main endpoint (filtered locally)",
		zap.String("userPubkey", userPubkey),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalSwaps", len(allSwapsFromMain)))

	for i := range allSwapsFromMain {
		swap := &allSwapsFromMain[i]

		isBuy := swap.AssetInAddress == NativeTokenAddress && swap.AssetOutAddress != NativeTokenAddress

		if isBuy && swap.Timestamp != "" {
			createdTime, err := time.Parse(time.RFC3339, swap.Timestamp)
			if err != nil {
				LogDebug("Failed to parse timestamp from main endpoint",
					zap.String("timestamp", swap.Timestamp),
					zap.String("swapID", swap.ID),
					zap.Error(err))
				continue
			}

			if firstBuySwap == nil || createdTime.Before(firstBuyTime) {
				firstBuySwap = swap
				firstBuyTime = createdTime
				LogDebug("Found earlier buy swap from main endpoint",
					zap.String("swapID", swap.ID),
					zap.String("timestamp", swap.Timestamp),
					zap.Time("parsedTime", createdTime),
					zap.Int("index", i))
			}
		}
	}

	if firstBuySwap != nil && firstBuySwap.Timestamp != "" {
		moscowLocation, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			LogDebug("Failed to load Moscow timezone, using UTC", zap.Error(err))
			moscowLocation = time.UTC
		}
		moscowTime := firstBuyTime.In(moscowLocation)
		formattedDate := moscowTime.Format("2006-01-02 15:04")

		LogDebug("First buy swap found via main endpoint and formatted",
			zap.String("userPubkey", userPubkey),
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.String("swapID", firstBuySwap.ID),
			zap.String("timestamp", firstBuySwap.Timestamp),
			zap.String("formattedDate", formattedDate),
			zap.Time("moscowTime", moscowTime),
			zap.Int("totalSwapsFromMain", len(allSwapsFromMain)))
		return formattedDate, nil
	}

	LogDebug("No buy swap with timestamp found for pool (tried both endpoints)",
		zap.String("userPubkey", userPubkey),
		zap.String("poolLpPublicKey", poolLpPublicKey),
		zap.Int("totalSwapsFromUserEndpoint", len(userSwapsResp.Swaps)),
		zap.Int("totalSwapsFromMainEndpoint", len(allSwapsFromMain)))
	return "", nil
}
