package sdk

import (
	"time"

	pb "surveyor/sdk/go/gen/probev1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewErrorEnvelope(meta *pb.ProbeMetadata, host, ip, platform, trigger string, err error) *pb.Envelope {
	return &pb.Envelope{
		Probe:             meta,
		Host:              host,
		IpAddress:         ip,
		Platform:          platform,
		CollectedAt:       timestamppb.New(time.Now()),
		CollectionTrigger: trigger,
		Status:            pb.CollectStatus_COLLECT_STATUS_ERROR,
		ErrorDetail:       err.Error(),
	}
}
