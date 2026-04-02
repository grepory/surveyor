package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "test.key")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	err = pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	require.NoError(t, err)

	return path, key
}

func TestBYOKSigner_Sign(t *testing.T) {
	keyPath, _ := generateTestKey(t)

	signer, err := NewBYOKSigner(keyPath)
	require.NoError(t, err)

	data := []byte(`{"test": "declaration"}`)
	result, err := signer.Sign(context.Background(), data)
	require.NoError(t, err)

	hash := sha256.Sum256(data)
	expectedHash := hex.EncodeToString(hash[:])
	assert.Equal(t, expectedHash, result.ContentHash)
	assert.NotEmpty(t, result.Signature)
	assert.Empty(t, result.Certificate)
	assert.Empty(t, result.RekorLogEntry)
}

func TestBYOKSigner_InvalidKeyPath(t *testing.T) {
	_, err := NewBYOKSigner("/nonexistent/key.pem")
	assert.Error(t, err)
}

func TestBYOKSigner_DifferentDataProducesDifferentSignature(t *testing.T) {
	keyPath, _ := generateTestKey(t)
	signer, err := NewBYOKSigner(keyPath)
	require.NoError(t, err)

	r1, err := signer.Sign(context.Background(), []byte("data1"))
	require.NoError(t, err)

	r2, err := signer.Sign(context.Background(), []byte("data2"))
	require.NoError(t, err)

	assert.NotEqual(t, r1.ContentHash, r2.ContentHash)
	assert.NotEqual(t, r1.Signature, r2.Signature)
}

func TestKeylessSigner_Construction(t *testing.T) {
	s := NewKeylessSigner(WithRekorURL("https://rekor.example.com"))
	assert.NotNil(t, s)
	assert.Equal(t, "https://rekor.example.com", s.rekorURL)
}
