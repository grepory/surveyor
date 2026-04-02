package cdx

import (
	"encoding/json"
	"testing"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "surveyor/sdk/go/gen/probev1"
)

func TestSerialize_SuccessEnvelope(t *testing.T) {
	env := &pb.Envelope{
		Probe: &pb.ProbeMetadata{
			Id:           "aws-vpc-default-sg",
			Version:      "0.1.0",
			EvidenceType: "network-boundary-controls",
		},
		Host:              "prod-collector-01",
		IpAddress:         "10.0.0.1",
		Platform:          "linux",
		CollectedAt:       timestamppb.New(time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)),
		ContentType:       "application/json",
		Payload:           []byte(`{"vpc_id":"vpc-123","rules":[]}`),
		CollectionTrigger: "scheduled",
		Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
	}

	controls := []string{"SC-7", "AC-17", "CM-7"}

	data, err := Serialize(env, controls)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, "CycloneDX", raw["bomFormat"])
	assert.Equal(t, "1.6", raw["specVersion"])
}

func TestSerialize_ErrorEnvelope(t *testing.T) {
	env := &pb.Envelope{
		Probe: &pb.ProbeMetadata{
			Id:           "aws-vpc-default-sg",
			Version:      "0.1.0",
			EvidenceType: "network-boundary-controls",
		},
		Host:              "prod-collector-01",
		CollectedAt:       timestamppb.New(time.Now()),
		CollectionTrigger: "scheduled",
		Status:            pb.CollectStatus_COLLECT_STATUS_ERROR,
		ErrorDetail:       "access denied for role arn:aws:iam::123:role/x",
	}

	data, err := Serialize(env, []string{"SC-7"})
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
}

func TestSerialize_RoundTrip(t *testing.T) {
	env := &pb.Envelope{
		Probe: &pb.ProbeMetadata{
			Id:           "test-probe",
			Version:      "1.0.0",
			EvidenceType: "test-evidence",
		},
		Host:              "host1",
		IpAddress:         "10.0.0.1",
		Platform:          "linux",
		CollectedAt:       timestamppb.New(time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)),
		ContentType:       "application/json",
		Payload:           []byte(`{"key":"value"}`),
		CollectionTrigger: "manual",
		Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
	}

	data, err := Serialize(env, []string{"SC-7"})
	require.NoError(t, err)

	bom := new(cdx.BOM)
	require.NoError(t, json.Unmarshal(data, bom))

	assert.Equal(t, cdx.SpecVersion1_6, bom.SpecVersion)
	assert.Equal(t, cdx.BOMFormat, bom.BOMFormat)
	require.NotNil(t, bom.Metadata)
}
