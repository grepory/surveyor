package sdk

import (
	"context"
	"testing"

	pb "surveyor/sdk/go/gen/probev1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeFunc_Metadata(t *testing.T) {
	meta := &pb.ProbeMetadata{
		Id:                "test-probe",
		Version:           "0.1.0",
		EvidenceType:      "test-evidence",
		SupportsStreaming: false,
	}
	p := &ProbeFunc{Meta: meta}

	got, err := p.Metadata(context.Background(), &pb.MetadataRequest{})
	require.NoError(t, err)
	assert.Equal(t, meta, got)
}

func TestProbeFunc_Collect(t *testing.T) {
	expected := &pb.Envelope{Host: "testhost"}
	p := &ProbeFunc{
		Meta: &pb.ProbeMetadata{Id: "test"},
		OnCollect: func(ctx context.Context, req *pb.CollectRequest) (*pb.Envelope, error) {
			return expected, nil
		},
	}

	resp, err := p.Collect(context.Background(), &pb.CollectRequest{})
	require.NoError(t, err)
	assert.Equal(t, expected, resp.Envelope)
}

func TestProbeFunc_Collect_NotImplemented(t *testing.T) {
	p := &ProbeFunc{Meta: &pb.ProbeMetadata{Id: "test"}}

	_, err := p.Collect(context.Background(), &pb.CollectRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support Collect")
}
