package telemetry

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type emitFunc func(ctx context.Context, name string, attrs map[string]interface{})

type Emitter struct {
	emit     emitFunc
	provider *otellog.LoggerProvider
	logger   log.Logger
}

func NewEmitter(ctx context.Context, endpoint string) (*Emitter, error) {
	// Strip scheme from URL — gRPC exporter expects host:port, not a full URL.
	grpcEndpoint := endpoint
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		grpcEndpoint = u.Host
	}

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(grpcEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTel log exporter: %w", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("surveyor-agent"),
		semconv.ServiceVersion("0.1.0"),
	)

	provider := otellog.NewLoggerProvider(
		otellog.WithProcessor(otellog.NewBatchProcessor(exporter)),
		otellog.WithResource(res),
	)

	logger := provider.Logger("surveyor")

	e := &Emitter{
		provider: provider,
		logger:   logger,
	}
	e.emit = e.otelEmit
	return e, nil
}

func (e *Emitter) Shutdown(ctx context.Context) error {
	if e.provider != nil {
		return e.provider.Shutdown(ctx)
	}
	return nil
}

func (e *Emitter) otelEmit(ctx context.Context, name string, attrs map[string]interface{}) {
	var record log.Record
	record.SetBody(log.StringValue(name))

	kvs := make([]log.KeyValue, 0, len(attrs)+1)
	kvs = append(kvs, log.String("event.name", name))
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			kvs = append(kvs, log.String(k, val))
		case int:
			kvs = append(kvs, log.Int(k, val))
		case int64:
			kvs = append(kvs, log.Int64(k, val))
		case bool:
			kvs = append(kvs, log.Bool(k, val))
		default:
			kvs = append(kvs, log.String(k, fmt.Sprintf("%v", val)))
		}
	}
	record.AddAttributes(kvs...)
	e.logger.Emit(ctx, record)
}

// Attribute types for each event.

type EvidenceCollectedAttrs struct {
	ProbeID      string
	Host         string
	EvidenceType string
	Status       string
	ContentHash  string
}

type EvidenceSignedAttrs struct {
	ProbeID     string
	ContentHash string
	RekorEntry  string
}

type EvidenceStoredAttrs struct {
	ProbeID     string
	ContentHash string
	S3Key       string
}

type SigningFailedAttrs struct {
	ProbeID string
	Error   string
}

type ProbeCrashedAttrs struct {
	ProbeID        string
	ExitCode       int
	RestartAttempt int
}

type ProbeDegradedAttrs struct {
	ProbeID             string
	ConsecutiveFailures int
}

func (e *Emitter) EmitEvidenceCollected(ctx context.Context, a EvidenceCollectedAttrs) {
	e.emit(ctx, EventEvidenceCollected, map[string]interface{}{
		"probe_id":      a.ProbeID,
		"host":          a.Host,
		"evidence_type": a.EvidenceType,
		"status":        a.Status,
		"content_hash":  a.ContentHash,
	})
}

func (e *Emitter) EmitEvidenceSigned(ctx context.Context, a EvidenceSignedAttrs) {
	e.emit(ctx, EventEvidenceSigned, map[string]interface{}{
		"probe_id":     a.ProbeID,
		"content_hash": a.ContentHash,
		"rekor_entry":  a.RekorEntry,
	})
}

func (e *Emitter) EmitEvidenceStored(ctx context.Context, a EvidenceStoredAttrs) {
	e.emit(ctx, EventEvidenceStored, map[string]interface{}{
		"probe_id":     a.ProbeID,
		"content_hash": a.ContentHash,
		"s3_key":       a.S3Key,
	})
}

func (e *Emitter) EmitSigningFailed(ctx context.Context, a SigningFailedAttrs) {
	e.emit(ctx, EventSigningFailed, map[string]interface{}{
		"probe_id": a.ProbeID,
		"error":    a.Error,
	})
}

func (e *Emitter) EmitProbeCrashed(ctx context.Context, a ProbeCrashedAttrs) {
	e.emit(ctx, EventProbeCrashed, map[string]interface{}{
		"probe_id":        a.ProbeID,
		"exit_code":       a.ExitCode,
		"restart_attempt": a.RestartAttempt,
	})
}

func (e *Emitter) EmitProbeDegraded(ctx context.Context, a ProbeDegradedAttrs) {
	e.emit(ctx, EventProbeDegraded, map[string]interface{}{
		"probe_id":             a.ProbeID,
		"consecutive_failures": a.ConsecutiveFailures,
	})
}
