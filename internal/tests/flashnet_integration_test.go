//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	flashnet "spark-wallet/internal/clients_api/flashnet"
)

// localTokenFile is a minimal view over token.json for integration tests.
type localTokenFile struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
	PublicKey   string `json:"publicKey"`
}

// findRepoRoot walks up from current working dir until it finds go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("repo root not found (go.mod)")
}

// loadLocalTokenFile is a helper for integration test only.
func loadLocalTokenFile(dataDir string) (*localTokenFile, error) {
	b, err := os.ReadFile(filepath.Join(dataDir, "token.json"))
	if err != nil {
		return nil, err
	}
	var tf localTokenFile
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

func tokenValid(tf *localTokenFile) bool {
	if tf == nil || tf.AccessToken == "" {
		return false
	}
	exp := tf.ExpiresAt
	if exp == 0 {
		// best effort: decode exp from JWT
		if jwtExp, err := flashnet.GetTokenExpirationTime(tf.AccessToken); err == nil {
			exp = jwtExp
		}
	}
	// small skew for safety
	return exp > time.Now().Add(30*time.Second).Unix()
}

// TestIntegration_Flashnet_ChallengeVerifySwaps:
// - Always checks /auth/challenge endpoint
// - Ensures valid token: uses existing token.json if valid, otherwise refreshes using spark-cli sign script + /auth/verify
// - Calls /swaps with JWT
func TestIntegration_Flashnet_ChallengeVerifySwaps(t *testing.T) {
	dataDir := "data_in"
	network := os.Getenv("NETWORK")
	if network == "" {
		network = "mainnet"
	}

	// Prefer PUBLIC_KEY from env, otherwise try token.json.
	publicKey := os.Getenv("PUBLIC_KEY")
	if publicKey == "" {
		if tf, err := loadLocalTokenFile(dataDir); err == nil && tf.PublicKey != "" {
			publicKey = tf.PublicKey
		}
	}
	if publicKey == "" {
		t.Skip("PUBLIC_KEY is not set and token.json not found; cannot run Flashnet integration test")
	}

	c := flashnet.NewAMMClient(network)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	// 1) /auth/challenge must work (public endpoint)
	if _, err := c.GetChallenge(ctx, publicKey); err != nil {
		t.Fatalf("GetChallenge failed: %v", err)
	}

	// 2) Ensure valid token in memory (use existing if valid, else refresh)
	tfLoaded, err := flashnet.LoadTokenFromFile(dataDir)
	if err == nil && tfLoaded != nil && tfLoaded.AccessToken != "" {
		// use flashnet's own struct if present and valid
		if exp := tfLoaded.ExpiresAt; exp == 0 {
			if jwtExp, err := flashnet.GetTokenExpirationTime(tfLoaded.AccessToken); err == nil {
				tfLoaded.ExpiresAt = jwtExp
			}
		}
		if tfLoaded.ExpiresAt > time.Now().Add(30*time.Second).Unix() {
			c.SetJWT(tfLoaded.AccessToken)
		} else {
			tfLoaded = nil
		}
	}

	if tfLoaded == nil {
		// Refresh token: challenge -> sign -> verify
		if _, err := c.GetChallengeAndSave(ctx, dataDir, publicKey); err != nil {
			t.Fatalf("GetChallengeAndSave failed: %v", err)
		}

		// Run spark-cli signer to produce signature.json for current challenge.json
		if _, err := exec.LookPath("node"); err != nil {
			t.Skip("node not found in PATH; cannot run spark-cli sign script")
		}
		root, err := findRepoRoot()
		if err != nil {
			t.Fatalf("findRepoRoot failed: %v", err)
		}

		signCtx, signCancel := context.WithTimeout(ctx, 60*time.Second)
		defer signCancel()

		cmd := exec.CommandContext(signCtx, "node", filepath.Join("spark-cli", "sign-challenge.mjs"))
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("sign-challenge.mjs failed: %v\noutput:\n%s", err, string(out))
		}

		sig, err := flashnet.LoadSignatureFromFile(dataDir)
		if err != nil {
			t.Fatalf("LoadSignatureFromFile failed: %v", err)
		}
		if sig.Signature == "" {
			t.Fatalf("signature.json has empty signature after signing")
		}

		if _, err := c.VerifySignatureAndSave(ctx, dataDir, sig.PublicKey, sig.Signature); err != nil {
			t.Fatalf("VerifySignatureAndSave failed: %v", err)
		}

		tfLoaded, err = flashnet.LoadTokenFromFile(dataDir)
		if err != nil || tfLoaded == nil || tfLoaded.AccessToken == "" {
			t.Fatalf("token.json not valid after refresh; err=%v", err)
		}
		if tfLoaded.ExpiresAt == 0 {
			if jwtExp, err := flashnet.GetTokenExpirationTime(tfLoaded.AccessToken); err == nil {
				tfLoaded.ExpiresAt = jwtExp
			}
		}
		if tfLoaded.ExpiresAt <= time.Now().Add(30*time.Second).Unix() {
			t.Fatalf("token.json expiration not valid after refresh")
		}
		c.SetJWT(tfLoaded.AccessToken)
	}

	// 3) /swaps must be accessible with JWT
	limit := 1
	resp, err := c.GetSwaps(ctx, flashnet.GetSwapsOptions{Limit: &limit})
	if err != nil {
		t.Fatalf("GetSwaps failed: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetSwaps returned nil response")
	}
}
