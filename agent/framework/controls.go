package framework

import (
	"surveyor/agent/config"
)

var defaults = map[string][]string{
	"network-boundary-controls": {"SC-7", "AC-17", "CM-7"},
	"iam-access-controls":       {"AC-2", "AC-3", "AC-6"},
	"encryption-at-rest":        {"SC-28"},
	"audit-logging":             {"AU-2", "AU-3", "AU-12"},
}

type Resolver struct {
	merged map[string][]string
}

func NewResolver(overrides map[string]config.MappingOverride) *Resolver {
	merged := make(map[string][]string, len(defaults))
	for k, v := range defaults {
		cp := make([]string, len(v))
		copy(cp, v)
		merged[k] = cp
	}

	for evidenceType, override := range overrides {
		if override.Replace {
			merged[evidenceType] = override.Controls
		} else {
			merged[evidenceType] = append(merged[evidenceType], override.Add...)
		}
	}

	return &Resolver{merged: merged}
}

func (r *Resolver) Resolve(evidenceType string) []string {
	return r.merged[evidenceType]
}
