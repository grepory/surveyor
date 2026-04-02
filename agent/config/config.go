package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent           AgentConfig               `yaml:"agent"`
	Storage         StorageConfig             `yaml:"storage"`
	Signing         SigningConfig             `yaml:"signing"`
	ControlMappings map[string]MappingOverride `yaml:"control_mappings,omitempty"`
	Probes          []ProbeConfig             `yaml:"probes"`
}

type AgentConfig struct {
	LogLevel               string `yaml:"log_level"`
	OTelEndpoint           string `yaml:"otel_endpoint"`
	ShutdownTimeoutSeconds int    `yaml:"shutdown_timeout_seconds"`
}

type StorageConfig struct {
	Type       string `yaml:"type"`
	Bucket     string `yaml:"bucket"`
	Region     string `yaml:"region"`
	ObjectLock bool   `yaml:"object_lock"`
}

type SigningConfig struct {
	Mode     string `yaml:"mode"`
	KeyPath  string `yaml:"key_path,omitempty"`
	RekorURL string `yaml:"rekor_url,omitempty"`
}

type MappingOverride struct {
	Add      []string `yaml:"add,omitempty"`
	Replace  bool     `yaml:"replace,omitempty"`
	Controls []string `yaml:"controls,omitempty"`
}

type ProbeConfig struct {
	ID       string                 `yaml:"id"`
	Binary   string                 `yaml:"binary"`
	Schedule ScheduleConfig         `yaml:"schedule"`
	Config   map[string]interface{} `yaml:"config,omitempty"`
}

type ScheduleConfig struct {
	MinInterval int `yaml:"min_interval"`
	MaxInterval int `yaml:"max_interval"`
}

func Parse(data []byte) (*Config, error) {
	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	return Parse(data)
}

func (c *Config) Validate() error {
	if c.Agent.OTelEndpoint == "" {
		return fmt.Errorf("agent.otel_endpoint is required")
	}
	u, err := url.Parse(c.Agent.OTelEndpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("agent.otel_endpoint is not a valid URL: %q", c.Agent.OTelEndpoint)
	}
	if c.Storage.Bucket == "" {
		return fmt.Errorf("storage.bucket is required")
	}
	if c.Storage.Region == "" {
		return fmt.Errorf("storage.region is required")
	}
	switch c.Signing.Mode {
	case "keyless":
	case "byok":
		if c.Signing.KeyPath == "" {
			return fmt.Errorf("signing.key_path is required when signing mode is byok")
		}
		if _, err := os.Stat(c.Signing.KeyPath); err != nil {
			return fmt.Errorf("signing.key_path %q does not exist: %w", c.Signing.KeyPath, err)
		}
	default:
		return fmt.Errorf("signing mode must be \"keyless\" or \"byok\", got %q", c.Signing.Mode)
	}
	if len(c.Probes) == 0 {
		return fmt.Errorf("at least one probe must be configured in probes")
	}
	for i, p := range c.Probes {
		if p.ID == "" {
			return fmt.Errorf("probes[%d].id is required", i)
		}
		fi, err := os.Stat(p.Binary)
		if err != nil {
			return fmt.Errorf("probes[%d].binary %q does not exist: %w", i, p.Binary, err)
		}
		if fi.Mode()&0o111 == 0 {
			return fmt.Errorf("probes[%d].binary %q is not executable", i, p.Binary)
		}
		if p.Schedule.MinInterval <= 0 {
			return fmt.Errorf("probes[%d].schedule.min_interval must be > 0", i)
		}
		if p.Schedule.MaxInterval <= 0 {
			return fmt.Errorf("probes[%d].schedule.max_interval must be > 0", i)
		}
		if p.Schedule.MinInterval >= p.Schedule.MaxInterval {
			return fmt.Errorf("probes[%d].schedule.min_interval (%d) must be < max_interval (%d)", i, p.Schedule.MinInterval, p.Schedule.MaxInterval)
		}
	}
	for evidenceType, m := range c.ControlMappings {
		if evidenceType == "" {
			return fmt.Errorf("control_mappings key must be a non-empty evidence type")
		}
		if m.Replace && len(m.Controls) == 0 {
			return fmt.Errorf("control_mappings[%s]: replace is true but controls list is empty", evidenceType)
		}
	}
	return nil
}
