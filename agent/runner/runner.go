package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	pb "surveyor/sdk/go/gen/probev1"
)

type ProbeRunner struct {
	binaryPath string
	logger     hclog.Logger

	mu       sync.Mutex
	client   *plugin.Client
	probe    pb.ProbeServiceClient
	metadata *pb.ProbeMetadata

	consecutiveFailures int
	degraded            bool
}

func New(binaryPath string, logger hclog.Logger) *ProbeRunner {
	return &ProbeRunner{
		binaryPath: binaryPath,
		logger:     logger,
	}
}

func (r *ProbeRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked(ctx)
}

func (r *ProbeRunner) startLocked(ctx context.Context) error {
	r.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          clientPluginMap(),
		Cmd:              exec.Command(r.binaryPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           r.logger,
	})

	rpcClient, err := r.client.Client()
	if err != nil {
		r.client.Kill()
		return fmt.Errorf("connecting to probe %s: %w", r.binaryPath, err)
	}

	raw, err := rpcClient.Dispense("probe")
	if err != nil {
		r.client.Kill()
		return fmt.Errorf("dispensing probe plugin %s: %w", r.binaryPath, err)
	}

	r.probe = raw.(pb.ProbeServiceClient)

	meta, err := r.probe.Metadata(ctx, &pb.MetadataRequest{})
	if err != nil {
		r.client.Kill()
		return fmt.Errorf("calling Metadata on probe %s: %w", r.binaryPath, err)
	}
	r.metadata = meta
	r.consecutiveFailures = 0
	return nil
}

func (r *ProbeRunner) Metadata() *pb.ProbeMetadata {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.metadata
}

func (r *ProbeRunner) IsDegraded() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.degraded
}

func (r *ProbeRunner) Collect(ctx context.Context, req *pb.CollectRequest) (*pb.Envelope, error) {
	r.mu.Lock()
	probe := r.probe
	r.mu.Unlock()

	if probe == nil {
		return nil, fmt.Errorf("probe %s is not running", r.binaryPath)
	}

	resp, err := probe.Collect(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Envelope, nil
}

func (r *ProbeRunner) CollectStream(ctx context.Context, req *pb.CollectRequest) ([]*pb.Envelope, error) {
	r.mu.Lock()
	probe := r.probe
	r.mu.Unlock()

	if probe == nil {
		return nil, fmt.Errorf("probe %s is not running", r.binaryPath)
	}

	stream, err := probe.CollectStream(ctx, req)
	if err != nil {
		return nil, err
	}

	var envelopes []*pb.Envelope
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return envelopes, err
		}
		envelopes = append(envelopes, resp.Envelope)
	}
	return envelopes, nil
}

func (r *ProbeRunner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		r.client.Kill()
		r.client = nil
	}
}

type CrashEvent struct {
	ProbeID        string
	ExitCode       int
	RestartAttempt int
	Degraded       bool
}

func (r *ProbeRunner) Restart(ctx context.Context, maxRetries int) CrashEvent {
	r.mu.Lock()
	r.consecutiveFailures++
	attempt := r.consecutiveFailures
	probeID := ""
	if r.metadata != nil {
		probeID = r.metadata.Id
	}

	if r.client != nil {
		r.client.Kill()
		r.client = nil
	}
	r.mu.Unlock()

	event := CrashEvent{
		ProbeID:        probeID,
		RestartAttempt: attempt,
	}

	if attempt > maxRetries {
		r.mu.Lock()
		r.degraded = true
		r.mu.Unlock()
		event.Degraded = true
		return event
	}

	backoff := time.Duration(1<<uint(attempt-1)) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}

	select {
	case <-time.After(backoff):
	case <-ctx.Done():
		event.Degraded = true
		return event
	}

	r.mu.Lock()
	err := r.startLocked(ctx)
	r.mu.Unlock()

	if err != nil {
		r.logger.Error("probe restart failed", "probe", probeID, "attempt", attempt, "error", err)
	}

	return event
}
