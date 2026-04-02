package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKey(t *testing.T) {
	ts := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	contentHash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	key := GenerateKey("aws-vpc-default-sg", "prod-collector-01", ts, contentHash)
	assert.Equal(t, "aws-vpc-default-sg/2026-04-02/prod-collector-01/1775131200-a1b2c3d4e5f6.cdx.json", key)
}

func TestGenerateKey_ShortHash(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	contentHash := "abcdef123456"

	key := GenerateKey("my-probe", "host1", ts, contentHash)
	assert.Equal(t, "my-probe/2026-01-15/host1/1768435200-abcdef123456.cdx.json", key)
}

func TestGenerateKey_ContentHashPrefix(t *testing.T) {
	ts := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	longHash := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	key := GenerateKey("probe", "host", ts, longHash)
	assert.Contains(t, key, "aabbccddeeff")
	assert.NotContains(t, key, "00112233")
}
