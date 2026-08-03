// Package config loads and validates the load balancer configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type BackendConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type HealthConfig struct {
	Path               string        `yaml:"path"`
	Interval           time.Duration `yaml:"interval"`
	Timeout            time.Duration `yaml:"timeout"`
	HealthyThreshold   int           `yaml:"healthy_threshold"`
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"`
	PassiveThreshold   int           `yaml:"passive_threshold"`
}

type ProxyConfig struct {
	MaxRetries int `yaml:"max_retries"`
}

type TransportConfig struct {
	MaxIdleConns        int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout"`
}

type Config struct {
	Listen          string          `yaml:"listen"`
	Algorithm       string          `yaml:"algorithm"`
	Backends        []BackendConfig `yaml:"backends"`
	Health          HealthConfig    `yaml:"health"`
	Proxy           ProxyConfig     `yaml:"proxy"`
	Transport       TransportConfig `yaml:"transport"`
	ShutdownTimeout time.Duration   `yaml:"shutdown_timeout"`
}

var validAlgorithms = map[string]bool{
	"round_robin":       true,
	"least_connections": true,
	"consistent_hash":   true,
	"p2c_ewma":          true,
}

// Load reads, parses, defaults, and validates a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.Algorithm == "" {
		c.Algorithm = "round_robin"
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 10 * time.Second
	}
	if c.Health.Path == "" {
		c.Health.Path = "/"
	}
	if c.Health.Interval == 0 {
		c.Health.Interval = 5 * time.Second
	}
	if c.Health.Timeout == 0 {
		c.Health.Timeout = 2 * time.Second
	}
	if c.Health.HealthyThreshold == 0 {
		c.Health.HealthyThreshold = 2
	}
	if c.Health.UnhealthyThreshold == 0 {
		c.Health.UnhealthyThreshold = 3
	}
	if c.Health.PassiveThreshold == 0 {
		c.Health.PassiveThreshold = 5
	}
	if c.Transport.MaxIdleConns == 0 {
		c.Transport.MaxIdleConns = 100
	}
	if c.Transport.MaxIdleConnsPerHost == 0 {
		c.Transport.MaxIdleConnsPerHost = 100
	}
	if c.Transport.IdleConnTimeout == 0 {
		c.Transport.IdleConnTimeout = 90 * time.Second
	}
	for i := range c.Backends {
		if c.Backends[i].Weight <= 0 {
			c.Backends[i].Weight = 1
		}
	}
}

// Validate checks the config is internally consistent.
func (c *Config) Validate() error {
	if len(c.Backends) == 0 {
		return fmt.Errorf("config: at least one backend is required")
	}
	if !validAlgorithms[c.Algorithm] {
		return fmt.Errorf("config: unknown algorithm %q", c.Algorithm)
	}
	for i, b := range c.Backends {
		if b.URL == "" {
			return fmt.Errorf("config: backend[%d] has empty url", i)
		}
	}
	return nil
}
