package framework

import (
	"testing"

	"surveyor/agent/config"

	"github.com/stretchr/testify/assert"
)

func TestResolve_DefaultsOnly(t *testing.T) {
	r := NewResolver(nil)
	controls := r.Resolve("network-boundary-controls")
	assert.Equal(t, []string{"SC-7", "AC-17", "CM-7"}, controls)
}

func TestResolve_UnknownEvidenceType(t *testing.T) {
	r := NewResolver(nil)
	controls := r.Resolve("unknown-type")
	assert.Empty(t, controls)
}

func TestResolve_AddMerge(t *testing.T) {
	overrides := map[string]config.MappingOverride{
		"network-boundary-controls": {
			Add: []string{"CA-3"},
		},
	}
	r := NewResolver(overrides)
	controls := r.Resolve("network-boundary-controls")
	assert.Equal(t, []string{"SC-7", "AC-17", "CM-7", "CA-3"}, controls)
}

func TestResolve_ReplaceEntirely(t *testing.T) {
	overrides := map[string]config.MappingOverride{
		"network-boundary-controls": {
			Replace:  true,
			Controls: []string{"SC-7", "CA-3"},
		},
	}
	r := NewResolver(overrides)
	controls := r.Resolve("network-boundary-controls")
	assert.Equal(t, []string{"SC-7", "CA-3"}, controls)
}

func TestResolve_AddToNonexistentDefault(t *testing.T) {
	overrides := map[string]config.MappingOverride{
		"custom-evidence": {
			Add: []string{"CP-9"},
		},
	}
	r := NewResolver(overrides)
	controls := r.Resolve("custom-evidence")
	assert.Equal(t, []string{"CP-9"}, controls)
}

func TestResolve_OtherDefaultsUnaffected(t *testing.T) {
	overrides := map[string]config.MappingOverride{
		"network-boundary-controls": {
			Replace:  true,
			Controls: []string{"CA-3"},
		},
	}
	r := NewResolver(overrides)
	controls := r.Resolve("iam-access-controls")
	assert.Equal(t, []string{"AC-2", "AC-3", "AC-6"}, controls)
}
