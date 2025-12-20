package flashnet

import (
	"context"
	"fmt"
	"time"

	"spark-wallet/internal/infra/log"

	"go.uber.org/zap"
)

// GetFirstBuySwap returns date/time (MSK) of first buy-swap for user+pool.
func GetFirstBuySwap(client *Client, userPubkey string, poolLpPublicKey string) (string, error) {
	if userPubkey == "" || poolLpPublicKey == "" {
		return "", fmt.Errorf("userPubkey and poolLpPublicKey are required")
	}

	if client == nil {
		log.LogDebug("Client is nil, cannot fetch first buy swap", zap.String("userPubkey", userPubkey), zap.String("poolLpPublicKey", poolLpPublicKey))
		return "", nil
	}

	ctx := context.Background()

	options := GetUserSwapsOptions{
		PoolLpPubkey: poolLpPublicKey,
		Limit:        1000,
		Sort:         "timestampAsc",
	}

	userSwapsResp, err := client.GetUserSwaps(ctx, userPubkey, options)
	if err != nil {
		log.LogDebug("Failed to fetch user swaps from Flashnet API",
			zap.String("userPubkey", userPubkey),
			zap.String("poolLpPublicKey", poolLpPublicKey),
			zap.Error(err))
		return "", nil
	}

	if userSwapsResp == nil || len(userSwapsResp.Swaps) == 0 {
		return "", nil
	}

	var firstBuySwap *Swap
	var firstBuyTime time.Time

	for i := range userSwapsResp.Swaps {
		swap := &userSwapsResp.Swaps[i]
		if swap.PoolLpPublicKey != poolLpPublicKey {
			continue
		}

		// buy = spend BTC, receive token
		isBuy := swap.AssetInAddress == NativeTokenAddress && swap.AssetOutAddress != NativeTokenAddress
		if !isBuy || swap.Timestamp == "" {
			continue
		}

		createdTime, err := time.Parse(time.RFC3339, swap.Timestamp)
		if err != nil {
			continue
		}

		if firstBuySwap == nil || createdTime.Before(firstBuyTime) {
			firstBuySwap = swap
			firstBuyTime = createdTime
		}
	}

	if firstBuySwap == nil || firstBuySwap.Timestamp == "" {
		return "", nil
	}

	moscowLocation, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		moscowLocation = time.UTC
	}
	return firstBuyTime.In(moscowLocation).Format("2006-01-02 15:04"), nil
}
