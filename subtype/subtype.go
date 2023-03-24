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
	Resource(name string) (resource.Resource, error)
	ReplaceAll(resources map[resource.Name]resource.Resource) error
	Add(resName resource.Name, res resource.Resource) error
	Remove(name resource.Name) error
	ReplaceOne(resName resource.Name, res resource.Resource) error
}

type subtypeSvc struct {
	mu         sync.RWMutex
	resources  map[string]resource.Resource
	shortNames map[string]string
	subtype    resource.Subtype
}

// New creates a new subtype service, which holds and replaces resources belonging to that subtype.
func New(subtype resource.Subtype, r map[resource.Name]resource.Resource) (Service, error) {
	s := &subtypeSvc{subtype: subtype}
	if err := s.ReplaceAll(r); err != nil {
		return nil, err
	}
	return s, nil
}

// Resource returns resource by name, if it exists.
func (s *subtypeSvc) Resource(name string) (resource.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if resource, ok := s.resources[name]; ok {
		return resource, nil
	}
	// looking for remote resource matching the name
	if resource, ok := s.resources[s.shortNames[name]]; ok {
		return resource, nil
	}
	return nil, resource.NewNotFoundError(resource.NameFromSubtype(s.subtype, name))
}

// ReplaceAll replaces all resources with r.
func (s *subtypeSvc) ReplaceAll(r map[resource.Name]resource.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources := make(map[string]resource.Resource, len(r))
	shortNames := make(map[string]string, len(r))
	s.resources = resources
	s.shortNames = shortNames
	for k, v := range r {
		if err := s.doAdd(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *subtypeSvc) Add(resName resource.Name, res resource.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doAdd(resName, res)
}

func (s *subtypeSvc) Remove(n resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doRemove(n)
}

func (s *subtypeSvc) ReplaceOne(resName resource.Name, res resource.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.doRemove(resName)
	if err != nil {
		return err
	}
	return s.doAdd(resName, res)
}

func (s *subtypeSvc) doAdd(resName resource.Name, res resource.Resource) error {
	if resName.Name == "" {
		return errors.Errorf("empty name used for resource: %s", resName)
	}
	name := resName.ShortName()

	_, exists := s.resources[name]
	if exists {
		return errors.Errorf("resource %s already exists", resName)
	}

	s.resources[name] = res
	shortcut := getShortcutName(name)
	if shortcut != name {
		if _, ok := s.shortNames[shortcut]; ok {
			s.shortNames[shortcut] = ""
		} else {
			s.shortNames[shortcut] = name
		}
	}
	return nil
}

func (s *subtypeSvc) doRemove(n resource.Name) error {
	name := n.ShortName()
	_, ok := s.resources[name]
	if !ok {
		return errors.Errorf("resource %s not found", name)
	}
	delete(s.resources, name)

	shortcut := getShortcutName(name)
	_, ok = s.shortNames[shortcut]
	if ok {
		delete(s.shortNames, shortcut)
	}

	// case: remote1:nameA and remote2:nameA both existed, and remote2:nameA is being deleted, restore shortcut to remote1:nameA
	for k := range s.resources {
		if shortcut == getShortcutName(k) && name != getShortcutName(k) {
			if _, ok := s.shortNames[shortcut]; ok {
				s.shortNames[shortcut] = ""
			} else {
				s.shortNames[shortcut] = k
			}
		}
	}
	return nil
}

func getShortcutName(name string) string {
	return name[strings.LastIndexAny(name, ":")+1:]
}

// LookupResource attempts to get specifically typed resource from the service.
func LookupResource[T resource.Resource](svc Service, name string) (T, error) {
	res, err := svc.Resource(name)
	if err != nil {
		var zero T
		return zero, err
	}
	return resource.AsType[T](res)
}
