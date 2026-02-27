package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services           map[string]ServiceConfig `yaml:"services"`
	Transmission       TransmissionConfig       `yaml:"transmission"`
	QBittorrent        QBittorrentConfig        `yaml:"qbittorrent"`
	MaxResponseSizeKB  int                      `yaml:"max_response_size_kb"`
	AllowDestructive   bool                     `yaml:"allow_destructive"`
}

type ServiceConfig struct {
	URL        string `yaml:"url"`
	APIKey     string `yaml:"api_key"`
	AuthMethod string `yaml:"auth_method"` // "header", "query", "basic"
	AuthHeader string `yaml:"auth_header"` // custom header name, defaults to X-Api-Key
	APIVersion string `yaml:"api_version"` // e.g. "/api/v3"
	OpenAPIURL string `yaml:"openapi_url"` // override spec URL
}

type TransmissionConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type QBittorrentConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "navigatorr", "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := &Config{
		Services: make(map[string]ServiceConfig),
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Apply defaults for known service types.
	for name, svc := range cfg.Services {
		if svc.AuthMethod == "" {
			if m, ok := DefaultAuthMethods[name]; ok {
				svc.AuthMethod = m
			} else {
				svc.AuthMethod = "header"
			}
		}
		if svc.AuthHeader == "" {
			if svc.AuthMethod == "header" {
				svc.AuthHeader = "X-Api-Key"
			}
		}
		if svc.APIVersion == "" {
			if v, ok := DefaultAPIVersions[name]; ok {
				svc.APIVersion = v
			}
		}
		if svc.OpenAPIURL == "" {
			if u, ok := DefaultOpenAPIURLs[name]; ok {
				svc.OpenAPIURL = u
			}
		}
		cfg.Services[name] = svc
	}

	// Default response size guard to 50KB if not set.
	if cfg.MaxResponseSizeKB <= 0 {
		cfg.MaxResponseSizeKB = 50
	}

	return cfg, nil
}
