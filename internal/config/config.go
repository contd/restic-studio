package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int                 `yaml:"version"`
	Locations map[string]Location `yaml:"locations"`
	Backends  map[string]Backend  `yaml:"backends"`
	Path      string              `yaml:"-"`
}

type Location struct {
	From    StringList     `yaml:"from"`
	To      StringList     `yaml:"to"`
	Options map[string]any `yaml:"options"`
}

type Backend struct {
	Type string            `yaml:"type"`
	Path string            `yaml:"path"`
	Key  string            `yaml:"key"`
	Env  map[string]string `yaml:"env"`
}

// StringList accepts Autorestic's scalar, inline-list, and block-list forms.
type StringList []string

func (s *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == "" {
			*s = nil
		} else {
			*s = []string{node.Value}
		}
		return nil
	case yaml.SequenceNode:
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("expected a scalar list item at line %d", item.Line)
			}
			values = append(values, item.Value)
		}
		*s = values
		return nil
	default:
		return fmt.Errorf("expected a string or list at line %d", node.Line)
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Autorestic config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse Autorestic config %q: invalid YAML syntax or field types; check indentation and scalar/list/map values", path)
	}
	cfg.Path = path
	if cfg.Locations == nil {
		cfg.Locations = make(map[string]Location)
	}
	if cfg.Backends == nil {
		cfg.Backends = make(map[string]Backend)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Autorestic config %q: %w", path, err)
	}
	return &cfg, nil
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".autorestic.yml"), nil
}

func (c *Config) Validate() error {
	if c.Version != 0 && c.Version != 2 {
		return fmt.Errorf("unsupported Autorestic config version %d; version 2 is required", c.Version)
	}
	if len(c.Locations) == 0 && len(c.Backends) == 0 {
		return fmt.Errorf("no locations or backends found")
	}
	for name, backend := range c.Backends {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("backend name must not be empty")
		}
		if strings.TrimSpace(backend.Path) == "" {
			return fmt.Errorf("backend %q is missing required field path", name)
		}
		if backend.Env == nil {
			backend.Env = make(map[string]string)
			c.Backends[name] = backend
		}
		for key, value := range backend.Env {
			if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "=\x00") {
				return fmt.Errorf("backend %q has an invalid environment variable name", name)
			}
			if strings.ContainsRune(value, '\x00') {
				return fmt.Errorf("backend %q environment variable %s contains an invalid value", name, key)
			}
		}
	}
	for name, location := range c.Locations {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("location name must not be empty")
		}
		if len(location.From) == 0 {
			return fmt.Errorf("location %q is missing required field from", name)
		}
		for _, source := range location.From {
			if strings.TrimSpace(source) == "" {
				return fmt.Errorf("location %q has an empty source path", name)
			}
		}
		for _, backend := range location.To {
			if _, ok := c.Backends[backend]; !ok {
				return fmt.Errorf("location %q references undefined backend %q", name, backend)
			}
		}
	}
	return nil
}

func (c *Config) LocationNames() []string {
	names := make([]string, 0, len(c.Locations))
	for name := range c.Locations {
		names = append(names, name)
	}
	return names
}

func (c *Config) BackendNames() []string {
	names := make([]string, 0, len(c.Backends))
	for name := range c.Backends {
		names = append(names, name)
	}
	return names
}
