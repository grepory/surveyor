//go:build integration

package integration

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"surveyor/agent/runner"
	pb "surveyor/sdk/go/gen/probev1"
)

func buildCrashProbe(t *testing.T) string {
	t.Helper()
	binPath := t.TempDir() + "/crashprobe"
	cmd := exec.Command("go", "build", "-o", binPath, "surveyor/internal/crashprobe")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build crash probe: %s", string(out))
	return binPath
}

func TestCrashRecovery_RestartWithBackoff(t *testing.T) {
	binPath := buildCrashProbe(t)
	logger := hclog.NewNullLogger()

	r := runner.New(binPath, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, r.Start(ctx))
	meta := r.Metadata()
	assert.Equal(t, "crash-probe", meta.Id)

	env, err := r.Collect(ctx, &pb.CollectRequest{
		ProbeId:           "crash-probe",
		CollectionTrigger: "scheduled",
	})
	require.NoError(t, err)
	assert.Equal(t, pb.CollectStatus_COLLECT_STATUS_SUCCESS, env.Status)

	// Second collect should fail because probe crashes
	_, err = r.Collect(ctx, &pb.CollectRequest{
		ProbeId:           "crash-probe",
		CollectionTrigger: "scheduled",
	})
	assert.Error(t, err)

	// Restart should succeed
	event := r.Restart(ctx, 5)
	assert.Equal(t, "crash-probe", event.ProbeID)
	assert.Equal(t, 1, event.RestartAttempt)
	assert.False(t, event.Degraded)
	assert.False(t, r.IsDegraded())
}

func TestCrashRecovery_DegradedAfterMaxRetries(t *testing.T) {
	logger := hclog.NewNullLogger()
	r := runner.New("/nonexistent/binary", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		event := r.Restart(ctx, 5)
		assert.Equal(t, i+1, event.RestartAttempt)
		assert.False(t, event.Degraded)
	}

	event := r.Restart(ctx, 5)
	assert.True(t, event.Degraded)
	assert.True(t, r.IsDegraded())
}
