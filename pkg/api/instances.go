package api

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadInstances reads an instances YAML file, unmarshals it, and validates.
func LoadInstances(filename string) (*InstancesConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading instances file: %w", err)
	}

	var cfg InstancesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing instances file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating instances file: %w", err)
	}

	return &cfg, nil
}

// Validate checks the instances configuration for errors.
func (c *InstancesConfig) Validate() error {
	if len(c.Instances) == 0 {
		return fmt.Errorf("instances list is empty")
	}

	names := make(map[string]bool)
	outputs := make(map[string]bool)

	for i, inst := range c.Instances {
		if inst.Name == "" {
			return fmt.Errorf("instance %d: name is required", i)
		}
		if inst.Output == "" {
			return fmt.Errorf("instance %q: output is required", inst.Name)
		}
		if names[inst.Name] {
			return fmt.Errorf("instance %q: duplicate name", inst.Name)
		}
		names[inst.Name] = true
		if outputs[inst.Output] {
			return fmt.Errorf("instance %q: duplicate output path %q", inst.Name, inst.Output)
		}
		outputs[inst.Output] = true
	}

	return nil
}
