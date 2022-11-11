// Package subtype contains a Service type that can be used to hold all resources of a certain subtype.
package subtype

import (
	"strings"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// Service defines an service that holds and replaces resources.
type Service interface {
	Resource(name string) interface{}
	Replace(resources map[resource.Name]interface{}) error
	Add(name resource.Name, iface interface{}) error
	Remove(name resource.Name) error
	ReplaceOne(n resource.Name, iface interface{}) error
}

type subtypeSvc struct {
	mu         sync.RWMutex
	resources  map[string]interface{}
	shortNames map[string]string
}

// New creates a new subtype service, which holds and replaces resources belonging to that subtype.
func New(r map[resource.Name]interface{}) (Service, error) {
	s := &subtypeSvc{}
	if err := s.Replace(r); err != nil {
		return nil, err
	}
	return s, nil
}

// Resource returns resource by name, if it exists.
func (s *subtypeSvc) Resource(name string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if resource, ok := s.resources[name]; ok {
		return resource
	}
	// looking for remote resource matching the name
	if resource, ok := s.resources[s.shortNames[name]]; ok {
		return resource
	}
	return nil
}

// Replace replaces all resources with r.
func (s *subtypeSvc) Replace(r map[resource.Name]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources := make(map[string]interface{}, len(r))
	shortNames := make(map[string]string, len(r))
	for n, v := range r {
		if n.Name == "" {
			return errors.Errorf("Empty name used for resource: %s", n)
		}
		name := n.ShortName()
		resources[name] = v
		shortcut := getShortcutName(name)
		if _, ok := shortNames[shortcut]; ok {
			shortNames[shortcut] = ""
		} else {
			shortNames[shortcut] = name
		}
		resources[name] = v
	}
	s.resources = resources
	s.shortNames = shortNames
	return nil
}

func (s *subtypeSvc) Add(n resource.Name, iface interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doAdd(n, iface)
}

func (s *subtypeSvc) Remove(n resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doRemove(n)
}

func (s *subtypeSvc) ReplaceOne(n resource.Name, iface interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.doRemove(n)
	if err != nil {
		return err
	}
	return s.doAdd(n, iface)
}

func (s *subtypeSvc) doAdd(n resource.Name, iface interface{}) error {
	if n.Name == "" {
		return errors.Errorf("empty name used for resource: %s", n)
	}

	_, exists := s.resources[n.ShortName()]
	if exists {
		return errors.Errorf("resource %s already exists", n)
	}

	name := n.ShortName()

	s.resources[name] = iface
	shortcut := getShortcutName(name)
	if _, ok := s.shortNames[shortcut]; ok {
		s.shortNames[shortcut] = ""
	} else {
		s.shortNames[shortcut] = name
	}
	return nil
}

func (s *subtypeSvc) doRemove(n resource.Name) error {
	name := n.ShortName()
	_, ok := s.resources[name]
	if ok {
		delete(s.resources, name)
	}

	shortcut := getShortcutName(name)
	_, ok = s.shortNames[shortcut]
	if ok {
		delete(s.resources, shortcut)
	}
	return nil
}

func getShortcutName(name string) string {
	return name[strings.LastIndexAny(name, ":")+1:]
}
