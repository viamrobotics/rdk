// Package subtype contains a Service type that can be used to hold all resources of a certain subtype.
package subtype

import (
	"sync"

	"github.com/pkg/errors"
	"go.viam.com/core/resource"
)

type Service interface {
	Resource(name string) interface{}
	Replace(resources map[resource.Name]interface{}) error
}

type subtypeSvc struct {
	mu        sync.RWMutex
	resources map[string]interface{}
}

func New(r map[resource.Name]interface{}) (Service, error) {
	s := &subtypeSvc{}
	if err := s.Replace(r); err != nil {
		return nil, err
	}
	return s, nil
}

// Resource returns resource by name, if it exists
func (s *subtypeSvc) Resource(name string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if resource, ok := s.resources[name]; ok {
		return resource
	}
	return nil
}

func (s *subtypeSvc) Replace(r map[resource.Name]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources := make(map[string]interface{}, len(r))
	for n, v := range r {
		if _, ok := resources[n.Name]; ok {
			return errors.Errorf("duplicate name in resources %s", n.Name)
		}
		resources[n.Name] = v
	}
	s.resources = resources
	return nil
}
