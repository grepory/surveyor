package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedEvent struct {
	name  string
	attrs map[string]interface{}
}

type recordingEmitter struct {
	events []recordedEvent
}

func (r *recordingEmitter) emit(ctx context.Context, name string, attrs map[string]interface{}) {
	r.events = append(r.events, recordedEvent{name: name, attrs: attrs})
}

func TestEmitter_EmitCollected(t *testing.T) {
	recorder := &recordingEmitter{}
	e := &Emitter{emit: recorder.emit}

	e.EmitEvidenceCollected(context.Background(), EvidenceCollectedAttrs{
		ProbeID:      "test-probe",
		Host:         "host1",
		EvidenceType: "test-evidence",
		Status:       "success",
		ContentHash:  "abc123",
	})

	require.Len(t, recorder.events, 1)
	assert.Equal(t, EventEvidenceCollected, recorder.events[0].name)
	assert.Equal(t, "test-probe", recorder.events[0].attrs["probe_id"])
	assert.Equal(t, "success", recorder.events[0].attrs["status"])
}

func TestEmitter_EmitSigned(t *testing.T) {
	recorder := &recordingEmitter{}
	e := &Emitter{emit: recorder.emit}

	e.EmitEvidenceSigned(context.Background(), EvidenceSignedAttrs{
		ProbeID:     "test-probe",
		ContentHash: "abc123",
		RekorEntry:  "rekor-entry-id",
	})

	require.Len(t, recorder.events, 1)
	assert.Equal(t, EventEvidenceSigned, recorder.events[0].name)
}

func TestEmitter_EmitProbeCrashed(t *testing.T) {
	recorder := &recordingEmitter{}
	e := &Emitter{emit: recorder.emit}

	e.EmitProbeCrashed(context.Background(), ProbeCrashedAttrs{
		ProbeID:        "test-probe",
		ExitCode:       1,
		RestartAttempt: 3,
	})

	require.Len(t, recorder.events, 1)
	assert.Equal(t, EventProbeCrashed, recorder.events[0].name)
	assert.Equal(t, 1, recorder.events[0].attrs["exit_code"])
}

func TestEmitter_EmitProbeDegraded(t *testing.T) {
	recorder := &recordingEmitter{}
	e := &Emitter{emit: recorder.emit}

	e.EmitProbeDegraded(context.Background(), ProbeDegradedAttrs{
		ProbeID:             "test-probe",
		ConsecutiveFailures: 5,
	})

	require.Len(t, recorder.events, 1)
	assert.Equal(t, EventProbeDegraded, recorder.events[0].name)
}
