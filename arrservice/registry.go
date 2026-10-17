package arrservice

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/jakenesler/navigatorr/config"
)

// Registry holds all configured services.
type Registry struct {
	services map[string]*Service
}

// NewRegistry creates a registry from config.
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		services: make(map[string]*Service),
	}
	// config.Load fills this in, but a Config built directly would leave it at
	// zero, and zero means no timeout to http.Client rather than the default.
	timeout := cfg.RequestTimeoutSeconds
	if timeout <= 0 {
		timeout = config.DefaultRequestTimeoutSeconds
	}
	// One client shared across services, so connection reuse is unchanged.
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	for name, svcCfg := range cfg.Services {
		svc := NewService(name, svcCfg)
		svc.client = client
		r.services[name] = svc
	}
	return r
}

// Get returns a service by name.
func (r *Registry) Get(name string) (*Service, error) {
	svc, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return svc, nil
}

// List returns all service names sorted.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// All returns all services.
func (r *Registry) All() map[string]*Service {
	return r.services
}
