//go:build integration

package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"surveyor/agent/runner"
	pb "surveyor/sdk/go/gen/probev1"
)

func buildReferenceProbe(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "aws-vpc-default-sg")
	cmd := exec.Command("go", "build", "-o", binPath, "surveyor/probes/aws-vpc-default-sg")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(out))
	return binPath
}

func TestPartialFailure_ErrorAndSuccessEnvelopes(t *testing.T) {
	binPath := buildReferenceProbe(t)
	logger := hclog.NewNullLogger()
	r := runner.New(binPath, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, r.Start(ctx))
	defer r.Stop()

	meta := r.Metadata()
	assert.Equal(t, "aws-vpc-default-sg", meta.Id)
	assert.True(t, meta.SupportsStreaming)

	configMap := map[string]interface{}{
		"regions": []interface{}{"us-east-1", "invalid-region-999"},
	}
	configStruct, err := structpb.NewStruct(configMap)
	require.NoError(t, err)

	req := &pb.CollectRequest{
		ProbeId:           "aws-vpc-default-sg",
		Config:            configStruct,
		CollectionTrigger: "manual",
	}

	envelopes, err := r.CollectStream(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, envelopes)

	var hasError bool
	for _, env := range envelopes {
		if env.Status == pb.CollectStatus_COLLECT_STATUS_ERROR {
			hasError = true
			assert.Contains(t, env.ErrorDetail, "invalid-region-999")
		}
	}
	assert.True(t, hasError, "expected at least one error envelope for invalid region")
}
