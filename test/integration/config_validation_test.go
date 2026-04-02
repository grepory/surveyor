//go:build integration

package integration

import (
	"testing"

	"surveyor/agent/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidation_MissingBucket(t *testing.T) {
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: ""
  region: us-east-1
signing:
  mode: keyless
probes: []
`
	cfg, err := config.Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket")
}

func TestConfigValidation_InvalidSigningMode(t *testing.T) {
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: test
  region: us-east-1
signing:
  mode: invalid
probes: []
`
	cfg, err := config.Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signing mode")
}
