package sdk

import (
	"errors"
	"testing"

	pb "surveyor/sdk/go/gen/probev1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorEnvelope(t *testing.T) {
	meta := &pb.ProbeMetadata{
		Id:           "test-probe",
		Version:      "1.0.0",
		EvidenceType: "test-evidence",
	}

	env := NewErrorEnvelope(meta, "host1", "10.0.0.1", "linux", "scheduled", errors.New("access denied"))

	require.NotNil(t, env)
	assert.Equal(t, "test-probe", env.Probe.Id)
	assert.Equal(t, "host1", env.Host)
	assert.Equal(t, "10.0.0.1", env.IpAddress)
	assert.Equal(t, "linux", env.Platform)
	assert.Equal(t, "scheduled", env.CollectionTrigger)
	assert.Equal(t, pb.CollectStatus_COLLECT_STATUS_ERROR, env.Status)
	assert.Equal(t, "access denied", env.ErrorDetail)
	assert.NotNil(t, env.CollectedAt)
	assert.Empty(t, env.Payload)
}
