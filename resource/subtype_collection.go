package resource

import (
	"strings"
	"sync"

	"github.com/pkg/errors"
)

// SubtypeCollection defines a collection of typed resources.
type SubtypeCollection[T Resource] interface {
	Resource(name string) (T, error)
	ReplaceAll(resources map[Name]T) error
	Add(resName Name, res T) error
	Remove(name Name) error
	ReplaceOne(resName Name, res T) error
}

type subtypeCollection[T Resource] struct {
	subtype Subtype

	mu         sync.RWMutex
	resources  map[string]T
	shortNames map[string]string
}

// NewEmptySubtypeCollection creates a new subtype collection, which holds and replaces resources belonging to that subtype.
func NewEmptySubtypeCollection[T Resource](subtype Subtype) SubtypeCollection[T] {
	return &subtypeCollection[T]{
		subtype:    subtype,
		resources:  map[string]T{},
		shortNames: map[string]string{},
	}
}

// NewSubtypeCollection creates a new subtype collection, which holds and replaces resources belonging to that subtype.
func NewSubtypeCollection[T Resource](subtype Subtype, r map[Name]T) (SubtypeCollection[T], error) {
	s := &subtypeCollection[T]{subtype: subtype}
	if err := s.ReplaceAll(r); err != nil {
		return nil, err
	}
	return s, nil
}

// Resource returns resource by name, if it exists.
func (s *subtypeCollection[T]) Resource(name string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if resource, ok := s.resources[name]; ok {
		return resource, nil
	}
	// looking for remote resource matching the name
	if resource, ok := s.resources[s.shortNames[name]]; ok {
		return resource, nil
	}
	var zero T
	return zero, NewNotFoundError(NameFromSubtype(s.subtype, name))
}

// ReplaceAll replaces all resources with r.
func (s *subtypeCollection[T]) ReplaceAll(r map[Name]T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources := make(map[string]T, len(r))
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

func (s *subtypeCollection[T]) Add(resName Name, res T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doAdd(resName, res)
}

func (s *subtypeCollection[T]) Remove(n Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doRemove(n)
}

func (s *subtypeCollection[T]) ReplaceOne(resName Name, res T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.doRemove(resName)
	if err != nil {
		return err
	}
	return s.doAdd(resName, res)
}

func (s *subtypeCollection[T]) doAdd(resName Name, res T) error {
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

func (s *subtypeCollection[T]) doRemove(n Name) error {
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
