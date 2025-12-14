package system_works

import "context"

// AMMClient for Flashnet API
type AMMClient interface {
	GetSwaps(ctx context.Context, opt GetSwapsOptions) (*SwapsResponse, error)
	GetUserSwaps(ctx context.Context, userPublicKey string, opt GetUserSwapsOptions) (*UserSwapsResponse, error)
	GetChallengeAndSave(ctx context.Context, dataDir string, publicKey string) (string, error)
	VerifySignatureAndSave(ctx context.Context, dataDir string, publicKey string, signature string) (string, error)
	SetJWT(token string)
	GetJWT() string
}
