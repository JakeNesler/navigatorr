package arrservice

import (
	"github.com/jakenesler/navigatorr/config"
)

// Service represents a configured *arr service.
type Service struct {
	Name       string
	Config     config.ServiceConfig
	Auth       AuthStrategy
	BaseURL    string // URL + APIVersion, e.g. "http://10.0.0.100:8989/api/v3"
}

// NewService creates a Service from config.
func NewService(name string, cfg config.ServiceConfig) *Service {
	svc := &Service{
		Name:    name,
		Config:  cfg,
		BaseURL: cfg.URL + cfg.APIVersion,
	}

	switch cfg.AuthMethod {
	case "query":
		svc.Auth = &QueryAuth{Param: "apikey", Key: cfg.APIKey}
	case "basic":
		// basic auth not typically used for *arr, but supported
		svc.Auth = &BasicAuth{Username: cfg.APIKey, Password: ""}
	default: // "header"
		header := cfg.AuthHeader
		if header == "" {
			header = "X-Api-Key"
		}
		svc.Auth = &HeaderAuth{Header: header, Key: cfg.APIKey}
	}

	return svc
}
