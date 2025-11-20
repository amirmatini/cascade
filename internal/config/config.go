package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Cache  CacheConfig  `yaml:"cache"`
	Egress EgressConfig `yaml:"egress"`
	Rules  RulesConfig  `yaml:"rules"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type CacheConfig struct {
	Directory      string        `yaml:"directory"`
	MaxSizeGB      float64       `yaml:"max_size_gb"`
	MinFileSizeKB  int64         `yaml:"min_file_size_kb"`
	MaxFileSizeMB  int64         `yaml:"max_file_size_mb"`
	DefaultTTL     time.Duration `yaml:"default_ttl"`
	BufferSizeKB   int           `yaml:"buffer_size_kb"`
	RespectHeaders bool          `yaml:"respect_headers"`
}

type EgressConfig struct {
	Enabled   bool   `yaml:"enabled"`
	ProxyType string `yaml:"proxy_type"` // http, socks5
	ProxyURL  string `yaml:"proxy_url"`
}

type RulesConfig struct {
	Passthrough      []string          `yaml:"passthrough"`
	HTTPSPassthrough []string          `yaml:"https_passthrough"`
	SpecialTTL       map[string]string `yaml:"special_ttl"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 3142
	}
	if cfg.Cache.Directory == "" {
		cfg.Cache.Directory = "/var/cache/cascade"
	}
	if cfg.Cache.MaxSizeGB <= 0 {
		cfg.Cache.MaxSizeGB = 100
	}
	if cfg.Cache.MinFileSizeKB == 0 {
		cfg.Cache.MinFileSizeKB = 1
	}
	if cfg.Cache.MaxFileSizeMB == 0 {
		cfg.Cache.MaxFileSizeMB = 10240
	}
	if cfg.Cache.DefaultTTL == 0 {
		cfg.Cache.DefaultTTL = 24 * time.Hour
	}
	if cfg.Cache.BufferSizeKB == 0 {
		cfg.Cache.BufferSizeKB = 64
	}

	for pattern := range cfg.Rules.SpecialTTL {
		ttlStr := cfg.Rules.SpecialTTL[pattern]
		_, err := time.ParseDuration(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL for pattern %s: %w", pattern, err)
		}
	}

	return &cfg, nil
}

func (c *Config) GetTTLForPath(path string) time.Duration {
	for pattern, ttlStr := range c.Rules.SpecialTTL {
		if matchPattern(path, pattern) {
			ttl, _ := time.ParseDuration(ttlStr)
			return ttl
		}
	}
	return c.Cache.DefaultTTL
}

func matchPattern(path, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(path, pattern[1:len(pattern)-1])
	}

	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(path, pattern[1:])
	}

	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, pattern[:len(pattern)-1])
	}

	return strings.Contains(path, pattern)
}
