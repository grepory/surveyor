package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfigYAML(binaryPath string) string {
	return `
agent:
  log_level: info
  otel_endpoint: "http://localhost:4317"
  shutdown_timeout_seconds: 30
storage:
  type: s3
  bucket: surveyor-evidence
  region: us-east-1
  object_lock: true
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + binaryPath + `
    schedule:
      min_interval: 60
      max_interval: 120
    config:
      key: value
`
}

func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
	return path
}

func TestParse_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	cfg, err := Parse([]byte(validConfigYAML(bin)))
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())
	assert.Equal(t, "info", cfg.Agent.LogLevel)
	assert.Equal(t, "http://localhost:4317", cfg.Agent.OTelEndpoint)
	assert.Equal(t, 30, cfg.Agent.ShutdownTimeoutSeconds)
	assert.Equal(t, "s3", cfg.Storage.Type)
	assert.Equal(t, "surveyor-evidence", cfg.Storage.Bucket)
	assert.Equal(t, "us-east-1", cfg.Storage.Region)
	assert.True(t, cfg.Storage.ObjectLock)
	assert.Equal(t, "keyless", cfg.Signing.Mode)
	assert.Len(t, cfg.Probes, 1)
	assert.Equal(t, "test-probe", cfg.Probes[0].ID)
}

func TestValidate_EmptyBucket(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: ""
  region: us-east-1
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "bucket")
}

func TestValidate_EmptyRegion(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: ""
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "region")
}

func TestValidate_InvalidSigningMode(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: invalid
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "signing mode")
}

func TestValidate_BYOKMissingKeyPath(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: byok
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "key_path")
}

func TestValidate_BYOKKeyPathNotExist(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: byok
  key_path: /nonexistent/key.pem
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "key_path")
}

func TestValidate_ProbeBinaryNotExist(t *testing.T) {
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: /nonexistent/binary
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "binary")
}

func TestValidate_ScheduleMinGteMax(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 120
      max_interval: 60
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "min_interval")
}

func TestValidate_ScheduleZeroInterval(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 0
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "min_interval")
}

func TestValidate_NoProbes(t *testing.T) {
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes: []
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "probes")
}

func TestValidate_OTelEndpointInvalidURL(t *testing.T) {
	dir := t.TempDir()
	bin := writeExecutable(t, dir, "probe")
	yaml := `
agent:
  otel_endpoint: "not-a-url"
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes:
  - id: test-probe
    binary: ` + bin + `
    schedule:
      min_interval: 60
      max_interval: 120
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	err = cfg.Validate()
	assert.ErrorContains(t, err, "otel_endpoint")
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte(`{invalid yaml`))
	assert.Error(t, err)
}

func TestParse_DisallowUnknownFields(t *testing.T) {
	yaml := `
agent:
  otel_endpoint: "http://localhost:4317"
  unknown_field: oops
storage:
  type: s3
  bucket: mybucket
  region: us-east-1
signing:
  mode: keyless
probes: []
`
	_, err := Parse([]byte(yaml))
	assert.Error(t, err)
}
