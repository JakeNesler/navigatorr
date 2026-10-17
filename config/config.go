package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services          map[string]ServiceConfig `yaml:"services"`
	Transmission      TransmissionConfig       `yaml:"transmission"`
	QBittorrent       QBittorrentConfig        `yaml:"qbittorrent"`
	SABnzbd           SABnzbdConfig            `yaml:"sabnzbd"`
	Queue             QueueConfig              `yaml:"queue"`
	MaxResponseSizeKB int                      `yaml:"max_response_size_kb"`
	AllowDestructive  bool                     `yaml:"allow_destructive"`
	// RequestTimeoutSeconds bounds a call_api request. Some endpoints spend
	// most of that budget before sending a byte, so a service with a slow
	// collection endpoint needs this raised rather than a smaller page.
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
}

type ServiceConfig struct {
	URL        string `yaml:"url"`
	APIKey     string `yaml:"api_key"`
	AuthMethod string `yaml:"auth_method"` // "header", "query", "basic"
	AuthHeader string `yaml:"auth_header"` // custom header name, defaults to X-Api-Key
	AuthPrefix string `yaml:"auth_prefix"` // prefix for the key value, e.g. "Bearer"
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

type SABnzbdConfig struct {
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"`
	URLBase string `yaml:"url_base"` // SABnzbd's own url_base, "/sabnzbd" by default
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "navigatorr", "config.yaml")
}

// QueueConfig controls the HTTP request queue. Listen is required to serve the
// HTTP endpoint; the MCP queue tools work regardless so an agent can always
// drain whatever has accumulated.
type QueueConfig struct {
	Listen string `yaml:"listen"` // e.g. ":8099"; empty disables the HTTP endpoint
	Token  string `yaml:"token"`  // bearer token; empty disables auth
	Path   string `yaml:"path"`   // queue file; defaults to ~/.config/navigatorr/queue.json
}

func DefaultQueuePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "navigatorr", "queue.json")
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
			if h, ok := DefaultAuthHeaders[name]; ok {
				svc.AuthHeader = h
			} else if svc.AuthMethod == "header" {
				svc.AuthHeader = "X-Api-Key"
			}
		}
		if svc.AuthPrefix == "" {
			if p, ok := DefaultAuthPrefixes[name]; ok {
				svc.AuthPrefix = p
			}
		}
		if svc.APIVersion == "" {
			if v, ok := DefaultAPIVersions[name]; ok {
				svc.APIVersion = v
			}
		}
		resolved, err := resolveURL(name, svc.URL)
		if err != nil {
			return nil, err
		}
		svc.URL = resolved
		if svc.OpenAPIURL == "" {
			if u, ok := DefaultOpenAPIURLs[name]; ok {
				svc.OpenAPIURL = u
			} else if p, ok := DefaultSelfHostedSpecPaths[name]; ok {
				// Runs after resolveURL so the spec URL inherits the scheme and
				// port that were filled in, rather than whatever partial host
				// the config happened to carry.
				svc.OpenAPIURL = strings.TrimSuffix(svc.URL, "/") + p
			}
		}
		cfg.Services[name] = svc
	}

	// Default response size guard to 50KB if not set.
	if cfg.MaxResponseSizeKB <= 0 {
		cfg.MaxResponseSizeKB = 50
	}

	if cfg.RequestTimeoutSeconds <= 0 {
		cfg.RequestTimeoutSeconds = DefaultRequestTimeoutSeconds
	}

	return cfg, nil
}

// resolveURL normalizes a service URL, filling in the scheme and the service's
// default port when they are absent. An omitted URL falls back to localhost;
// list_services reports the resolved URL, so a wrong guess is visible rather
// than a confusing failure on the first API call.
func resolveURL(name, raw string) (string, error) {
	port, known := DefaultPorts[name]

	if raw == "" {
		if !known {
			return "", fmt.Errorf("service %q: url is required (no default port for this service)", name)
		}
		return fmt.Sprintf("http://localhost:%d", port), nil
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("service %q: parsing url %q: %w", name, raw, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("service %q: url %q has no host", name, raw)
	}
	if u.Port() == "" && known {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	}

	return strings.TrimRight(u.String(), "/"), nil
}
