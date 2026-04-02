package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ContentHash returns the hex-encoded SHA-256 hash of the given data.
func ContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func GenerateKey(probeID, host string, collectedAt time.Time, contentHash string) string {
	date := collectedAt.UTC().Format("2006-01-02")
	timestamp := collectedAt.UTC().Unix()

	hashPrefix := contentHash
	if len(hashPrefix) > 12 {
		hashPrefix = hashPrefix[:12]
	}

	return fmt.Sprintf("%s/%s/%s/%d-%s.cdx.json", probeID, date, host, timestamp, hashPrefix)
}
