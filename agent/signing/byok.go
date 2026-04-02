package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
)

type BYOKSigner struct {
	key crypto.Signer
}

func NewBYOKSigner(keyPath string) (*BYOKSigner, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading signing key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", keyPath)
	}

	var key crypto.Signer
	switch block.Type {
	case "EC PRIVATE KEY":
		k, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key: %w", err)
		}
		key = k
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS8 private key: %w", err)
		}
		signer, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key does not implement crypto.Signer")
		}
		key = signer
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	return &BYOKSigner{key: key}, nil
}

func (s *BYOKSigner) Sign(ctx context.Context, declaration []byte) (SignedResult, error) {
	hash := sha256.Sum256(declaration)

	var sig []byte
	var err error

	switch k := s.key.(type) {
	case *ecdsa.PrivateKey:
		sig, err = ecdsa.SignASN1(rand.Reader, k, hash[:])
	default:
		sig, err = s.key.Sign(rand.Reader, hash[:], crypto.SHA256)
	}
	if err != nil {
		return SignedResult{}, fmt.Errorf("signing declaration: %w", err)
	}

	return SignedResult{
		Signature:   sig,
		ContentHash: hex.EncodeToString(hash[:]),
	}, nil
}
