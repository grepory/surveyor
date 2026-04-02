package cdx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"

	pb "surveyor/sdk/go/gen/probev1"
)

// Serialize converts an Envelope and its resolved controls into a CycloneDX 1.6 BOM JSON.
func Serialize(env *pb.Envelope, controls []string) ([]byte, error) {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.Version = 1
	bom.SpecVersion = cdx.SpecVersion1_6

	collectedAt := env.CollectedAt.AsTime()
	bom.Metadata = &cdx.Metadata{
		Timestamp: collectedAt.Format(time.RFC3339),
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    "surveyor",
					Version: "0.1.0",
				},
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    "surveyor-probe-" + env.Probe.Id,
					Version: env.Probe.Version,
				},
			},
		},
	}

	status := "success"
	if env.Status == pb.CollectStatus_COLLECT_STATUS_ERROR {
		status = "error"
	}

	properties := []cdx.Property{
		{Name: "surveyor:probe_id", Value: env.Probe.Id},
		{Name: "surveyor:evidence_type", Value: env.Probe.EvidenceType},
		{Name: "surveyor:host", Value: env.Host},
		{Name: "surveyor:ip_address", Value: env.IpAddress},
		{Name: "surveyor:platform", Value: env.Platform},
		{Name: "surveyor:collection_trigger", Value: env.CollectionTrigger},
		{Name: "surveyor:status", Value: status},
		{Name: "surveyor:content_type", Value: env.ContentType},
	}

	if env.ErrorDetail != "" {
		properties = append(properties, cdx.Property{
			Name: "surveyor:error_detail", Value: env.ErrorDetail,
		})
	}

	if len(env.Payload) > 0 {
		if env.ContentType == "application/json" {
			properties = append(properties, cdx.Property{
				Name: "surveyor:payload", Value: string(env.Payload),
			})
		} else {
			properties = append(properties, cdx.Property{
				Name: "surveyor:payload_base64", Value: base64.StdEncoding.EncodeToString(env.Payload),
			})
		}
	}

	// Store control mappings as properties since the CycloneDX Go library
	// may not support Declarations/Attestations types directly.
	for i, ctrl := range controls {
		properties = append(properties, cdx.Property{
			Name:  fmt.Sprintf("surveyor:control[%d]", i),
			Value: ctrl,
		})
	}

	component := cdx.Component{
		Type:       cdx.ComponentTypeData,
		Name:       fmt.Sprintf("%s-evidence-%s", env.Probe.Id, collectedAt.Format("20060102T150405Z")),
		Version:    env.Probe.Version,
		Properties: &properties,
	}
	bom.Components = &[]cdx.Component{component}

	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing CycloneDX BOM: %w", err)
	}
	return data, nil
}
