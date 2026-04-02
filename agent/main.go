package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"surveyor/agent/cdx"
	agentconfig "surveyor/agent/config"
	"surveyor/agent/framework"
	"surveyor/agent/runner"
	"surveyor/agent/scheduler"
	"surveyor/agent/signing"
	"surveyor/agent/storage"
	"surveyor/agent/telemetry"
	pb "surveyor/sdk/go/gen/probev1"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: surveyor-agent <config-file>")
		os.Exit(1)
	}

	// Load and validate config
	cfg, err := agentconfig.Load(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config validation error: %v\n", err)
		os.Exit(1)
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.LevelFromString(cfg.Agent.LogLevel),
		JSONFormat: true,
	})

	// Two separate contexts:
	// - schedCtx controls the scheduler loop (stop scheduling new runs)
	// - collectCtx controls in-flight collections (only cancelled on timeout)
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	collectCtx, collectCancel := context.WithCancel(context.Background())
	defer collectCancel()

	ctx := schedCtx // alias for init code

	// Init telemetry
	emitter, err := telemetry.NewEmitter(ctx, cfg.Agent.OTelEndpoint)
	if err != nil {
		logger.Error("failed to create OTel emitter", "error", err)
		os.Exit(1)
	}
	// Note: emitter.Shutdown is called explicitly during graceful shutdown below,
	// not via defer, to control ordering.

	// Init signer
	var signer signing.Signer
	switch cfg.Signing.Mode {
	case "keyless":
		var opts []signing.KeylessOption
		if cfg.Signing.RekorURL != "" {
			opts = append(opts, signing.WithRekorURL(cfg.Signing.RekorURL))
		}
		signer = signing.NewKeylessSigner(opts...)
	case "byok":
		signer, err = signing.NewBYOKSigner(cfg.Signing.KeyPath)
		if err != nil {
			logger.Error("failed to create BYOK signer", "error", err)
			os.Exit(1)
		}
	}

	// Init storage
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Storage.Region))
	if err != nil {
		logger.Error("failed to load AWS config", "error", err)
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(awsCfg)
	store := storage.NewS3Store(s3Client, cfg.Storage.Bucket)

	// Init control resolver
	resolver := framework.NewResolver(cfg.ControlMappings)

	// Launch probes
	runners := make(map[string]*runner.ProbeRunner)
	for _, probeCfg := range cfg.Probes {
		r := runner.New(probeCfg.Binary, logger.Named("probe."+probeCfg.ID))
		if err := r.Start(ctx); err != nil {
			logger.Error("failed to start probe", "probe", probeCfg.ID, "error", err)
			os.Exit(1)
		}
		meta := r.Metadata()
		logger.Info("probe started", "probe", meta.Id, "version", meta.Version,
			"evidence_type", meta.EvidenceType, "streaming", meta.SupportsStreaming)
		runners[probeCfg.ID] = r
	}

	// Build scheduler
	hostname, _ := os.Hostname()
	sched := scheduler.New()

	for _, probeCfg := range cfg.Probes {
		pc := probeCfg // capture
		r := runners[pc.ID]
		meta := r.Metadata()

		minInterval := time.Duration(pc.Schedule.MinInterval) * time.Second
		maxInterval := time.Duration(pc.Schedule.MaxInterval) * time.Second

		sched.Add(pc.ID, minInterval, maxInterval, func(_ context.Context) {
			// Use collectCtx (not the scheduler's context) so in-flight collections
			// can complete during graceful shutdown after schedCtx is cancelled.
			if r.IsDegraded() {
				return
			}

			// Build config struct
			configStruct, err := structpb.NewStruct(pc.Config)
			if err != nil {
				logger.Error("failed to convert probe config to struct", "probe", pc.ID, "error", err)
				return
			}

			req := &pb.CollectRequest{
				ProbeId:           pc.ID,
				Config:            configStruct,
				ScheduledAt:       timestamppb.Now(),
				CollectionTrigger: "scheduled",
			}

			var envelopes []*pb.Envelope
			var collectErr error

			if meta.SupportsStreaming {
				envelopes, collectErr = r.CollectStream(collectCtx, req)
			} else {
				env, err := r.Collect(collectCtx, req)
				if err != nil {
					collectErr = err
				} else {
					envelopes = []*pb.Envelope{env}
				}
			}

			if collectErr != nil {
				logger.Error("probe transport error, likely crashed", "probe", pc.ID, "error", collectErr)
				// Only restart on transport/process errors, not on logical ERROR envelopes
				event := r.Restart(collectCtx, 5)
				emitter.EmitProbeCrashed(collectCtx, telemetry.ProbeCrashedAttrs{
					ProbeID:        event.ProbeID,
					ExitCode:       event.ExitCode,
					RestartAttempt: event.RestartAttempt,
				})
				if event.Degraded {
					emitter.EmitProbeDegraded(collectCtx, telemetry.ProbeDegradedAttrs{
						ProbeID:             event.ProbeID,
						ConsecutiveFailures: event.RestartAttempt,
					})
				}
				return
			}

			// Process each envelope
			controls := resolver.Resolve(meta.EvidenceType)

			for _, env := range envelopes {
				status := "success"
				if env.Status == pb.CollectStatus_COLLECT_STATUS_ERROR {
					status = "error"
				}

				// Serialize to CycloneDX
				declaration, err := cdx.Serialize(env, controls)
				if err != nil {
					logger.Error("CycloneDX serialization failed", "probe", pc.ID, "error", err)
					continue
				}

				// Compute content hash for audit trail before signing
				contentHash := storage.ContentHash(declaration)

				// Emit evidence.collected before signing — this records the collection
				// attempt regardless of whether signing/storage succeeds.
				emitter.EmitEvidenceCollected(collectCtx, telemetry.EvidenceCollectedAttrs{
					ProbeID:      pc.ID,
					Host:         hostname,
					EvidenceType: meta.EvidenceType,
					Status:       status,
					ContentHash:  contentHash,
				})

				// Sign
				result, err := signer.Sign(collectCtx, declaration)
				if err != nil {
					logger.Error("signing failed", "probe", pc.ID, "error", err)
					emitter.EmitSigningFailed(collectCtx, telemetry.SigningFailedAttrs{
						ProbeID: pc.ID,
						Error:   err.Error(),
					})
					continue
				}

				emitter.EmitEvidenceSigned(collectCtx, telemetry.EvidenceSignedAttrs{
					ProbeID:     pc.ID,
					ContentHash: result.ContentHash,
					RekorEntry:  result.RekorLogEntry,
				})

				// Store
				key := storage.GenerateKey(pc.ID, hostname, env.CollectedAt.AsTime(), result.ContentHash)
				err = store.Put(collectCtx, key, declaration, storage.PutOptions{
					ObjectLock:    cfg.Storage.ObjectLock,
					RetentionDays: 365,
					ContentType:   "application/json",
				})
				if err != nil {
					logger.Error("storage failed", "probe", pc.ID, "error", err)
					continue
				}

				emitter.EmitEvidenceStored(collectCtx, telemetry.EvidenceStoredAttrs{
					ProbeID:     pc.ID,
					ContentHash: result.ContentHash,
					S3Key:       key,
				})
			}
		})
	}

	// Start scheduler
	sched.Start(schedCtx)
	logger.Info("agent started", "probes", len(cfg.Probes))

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig)

	// Graceful shutdown
	shutdownTimeout := 30 * time.Second
	if cfg.Agent.ShutdownTimeoutSeconds > 0 {
		shutdownTimeout = time.Duration(cfg.Agent.ShutdownTimeoutSeconds) * time.Second
	}

	schedCancel() // stop scheduling new runs

	// Wait for in-flight with timeout
	done := make(chan struct{})
	go func() {
		sched.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all in-flight collections completed")
	case <-time.After(shutdownTimeout):
		logger.Warn("shutdown timeout reached, cancelling in-flight collections")
		collectCancel()
	}

	// Kill probe subprocesses
	for id, r := range runners {
		r.Stop()
		logger.Info("probe stopped", "probe", id)
	}

	// Flush OTel
	if err := emitter.Shutdown(context.Background()); err != nil {
		logger.Error("OTel shutdown error", "error", err)
	}

	log.Println("agent shutdown complete")
}
