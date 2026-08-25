package provider

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"uuid"
)

// GeneratePKCE 生成 OAuth PKCE verifier 与 challenge。
func GeneratePKCE() PKCEChallenge {
	verifier := strings.ReplaceAll(uuid.NewV4().String(), "-", "")
	verifier += strings.ReplaceAll(uuid.NewV4().String(), "-", "")
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}
}
