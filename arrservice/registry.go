package arrservice

import (
	"fmt"
	"sort"

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
	for name, svcCfg := range cfg.Services {
		r.services[name] = NewService(name, svcCfg)
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
