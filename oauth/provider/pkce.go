package provider

import (
	"crypto/sha256"
	"encoding/base64"

	"github.com/liujitcn/go-utils/id"
)

// GeneratePKCE 生成 OAuth PKCE verifier 与 challenge。
func GeneratePKCE() PKCEChallenge {
	verifier := id.NewGUIDv4NoHyphen() + id.NewGUIDv4NoHyphen()
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}
}
