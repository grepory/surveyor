package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/timestamppb"

	sdk "surveyor/sdk/go"
	pb "surveyor/sdk/go/gen/probev1"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Info,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	meta := &pb.ProbeMetadata{
		Id:               "test-probe",
		Version:          "0.1.0",
		EvidenceType:     "test-evidence",
		SupportsStreaming: true,
	}

	hostname, _ := os.Hostname()

	probe := &sdk.ProbeFunc{
		Meta: meta,
		OnCollect: func(ctx context.Context, req *pb.CollectRequest) (*pb.Envelope, error) {
			return &pb.Envelope{
				Probe:             meta,
				Host:              hostname,
				Platform:          runtime.GOOS,
				CollectedAt:       timestamppb.New(time.Now()),
				ContentType:       "application/json",
				Payload:           []byte(`{"test": true}`),
				CollectionTrigger: req.CollectionTrigger,
				Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
			}, nil
		},
		OnStream: func(ctx context.Context, req *pb.CollectRequest, send func(*pb.Envelope) error) error {
			for i := 0; i < 3; i++ {
				if err := send(&pb.Envelope{
					Probe:             meta,
					Host:              hostname,
					Platform:          runtime.GOOS,
					CollectedAt:       timestamppb.New(time.Now()),
					ContentType:       "application/json",
					Payload:           []byte(fmt.Sprintf(`{"index": %d}`, i)),
					CollectionTrigger: req.CollectionTrigger,
					Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	sdk.Serve(probe, logger)
}
