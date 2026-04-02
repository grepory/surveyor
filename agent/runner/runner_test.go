package runner

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "surveyor/sdk/go/gen/probev1"
)

func buildTestProbe(t *testing.T) string {
	t.Helper()
	binPath := t.TempDir() + "/testprobe"
	cmd := exec.Command("go", "build", "-o", binPath, "surveyor/internal/testprobe")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build test probe: %s", string(out))
	return binPath
}

func TestProbeRunner_LaunchAndMetadata(t *testing.T) {
	binPath := buildTestProbe(t)
	logger := hclog.NewNullLogger()

	r := New(binPath, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := r.Start(ctx)
	require.NoError(t, err)
	defer r.Stop()

	meta := r.Metadata()
	assert.Equal(t, "test-probe", meta.Id)
	assert.Equal(t, "0.1.0", meta.Version)
	assert.Equal(t, "test-evidence", meta.EvidenceType)
	assert.True(t, meta.SupportsStreaming)
}

func TestProbeRunner_Collect(t *testing.T) {
	binPath := buildTestProbe(t)
	logger := hclog.NewNullLogger()

	r := New(binPath, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, r.Start(ctx))
	defer r.Stop()

	env, err := r.Collect(ctx, &pb.CollectRequest{
		ProbeId:           "test-probe",
		CollectionTrigger: "scheduled",
	})
	require.NoError(t, err)
	assert.Equal(t, pb.CollectStatus_COLLECT_STATUS_SUCCESS, env.Status)
	assert.Equal(t, "application/json", env.ContentType)
}

func TestProbeRunner_CollectStream(t *testing.T) {
	binPath := buildTestProbe(t)
	logger := hclog.NewNullLogger()

	r := New(binPath, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, r.Start(ctx))
	defer r.Stop()

	envelopes, err := r.CollectStream(ctx, &pb.CollectRequest{
		ProbeId:           "test-probe",
		CollectionTrigger: "scheduled",
	})
	require.NoError(t, err)
	assert.Len(t, envelopes, 3)
	for _, env := range envelopes {
		assert.Equal(t, pb.CollectStatus_COLLECT_STATUS_SUCCESS, env.Status)
	}
}
