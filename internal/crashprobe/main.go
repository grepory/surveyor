package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/timestamppb"

	sdk "surveyor/sdk/go"
	pb "surveyor/sdk/go/gen/probev1"
)

var collectCount atomic.Int32

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Info,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	crashAfter := 1
	if v := os.Getenv("CRASH_AFTER_N"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			crashAfter = n
		}
	}

	meta := &pb.ProbeMetadata{
		Id:                "crash-probe",
		Version:           "0.1.0",
		EvidenceType:      "test-evidence",
		SupportsStreaming: false,
	}

	hostname, _ := os.Hostname()

	probe := &sdk.ProbeFunc{
		Meta: meta,
		OnCollect: func(ctx context.Context, req *pb.CollectRequest) (*pb.Envelope, error) {
			n := collectCount.Add(1)
			if int(n) > crashAfter {
				os.Exit(1)
			}
			return &pb.Envelope{
				Probe:             meta,
				Host:              hostname,
				Platform:          runtime.GOOS,
				CollectedAt:       timestamppb.New(time.Now()),
				ContentType:       "application/json",
				Payload:           []byte(fmt.Sprintf(`{"collect_count":%d}`, n)),
				CollectionTrigger: req.CollectionTrigger,
				Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
			}, nil
		},
	}

	sdk.Serve(probe, logger)
}
