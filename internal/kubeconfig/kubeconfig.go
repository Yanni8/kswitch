// Package kubeconfig provides minimal parsing of kubeconfig files —
// just enough to validate pasted input and derive a display name.
package kubeconfig

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config is the subset of a kubeconfig that kswitch cares about.
type Config struct {
	APIVersion     string      `yaml:"apiVersion"`
	Kind           string      `yaml:"kind"`
	CurrentContext string      `yaml:"current-context"`
	Clusters       []namedItem `yaml:"clusters"`
	Contexts       []namedItem `yaml:"contexts"`
}

type namedItem struct {
	Name string `yaml:"name"`
}

// Parse validates that data is YAML shaped like a kubeconfig.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("not valid YAML: %w", err)
	}
	if c.Kind != "" && c.Kind != "Config" {
		return nil, fmt.Errorf("kind is %q, expected %q", c.Kind, "Config")
	}
	if len(c.Clusters) == 0 && len(c.Contexts) == 0 {
		return nil, errors.New("no clusters or contexts found; this does not look like a kubeconfig")
	}
	return &c, nil
}

// SuggestedName derives a name for the config: the current context,
// else the first context, else the first cluster.
func (c *Config) SuggestedName() string {
	if c.CurrentContext != "" {
		return c.CurrentContext
	}
	if len(c.Contexts) > 0 {
		return c.Contexts[0].Name
	}
	if len(c.Clusters) > 0 {
		return c.Clusters[0].Name
	}
	return ""
}
