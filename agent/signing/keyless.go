package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type KeylessSigner struct {
	rekorURL  string
	fulcioURL string
	issuerURL string
}

type KeylessOption func(*KeylessSigner)

func WithRekorURL(url string) KeylessOption {
	return func(s *KeylessSigner) { s.rekorURL = url }
}

func WithFulcioURL(url string) KeylessOption {
	return func(s *KeylessSigner) { s.fulcioURL = url }
}

func WithIssuerURL(url string) KeylessOption {
	return func(s *KeylessSigner) { s.issuerURL = url }
}

func NewKeylessSigner(opts ...KeylessOption) *KeylessSigner {
	s := &KeylessSigner{
		fulcioURL: "https://fulcio.sigstore.dev",
		rekorURL:  "https://rekor.sigstore.dev",
		issuerURL: "https://oauth2.sigstore.dev/auth",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *KeylessSigner) Sign(ctx context.Context, declaration []byte) (SignedResult, error) {
	hash := sha256.Sum256(declaration)
	_ = hex.EncodeToString(hash[:])
	return SignedResult{}, fmt.Errorf("keyless signing requires OIDC provider infrastructure (Fulcio: %s, Rekor: %s)", s.fulcioURL, s.rekorURL)
}
