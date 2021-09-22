// Package service contains a service type that can be used to hold information about a robot's components and services.
package service

import (
	"context"
	"sync"

	"github.com/go-errors/errors"

	"go.viam.com/core/resource"
)

// Service keeps track of all resources associated with a robot.
type Service struct {
	mu        sync.Mutex
	resources []resource.Name
}

// New creates a new Service struct and initializes the resource list with a metadata service.
func New() (*Service, error) {
	metadata, err := resource.New(resource.ResourceNamespaceCore, resource.ResourceTypeService, resource.ResourceSubtypeMetadata, "")
	if err != nil {
		return nil, err
	}
	resources := []resource.Name{metadata}

	return &Service{resources: resources}, nil
}

// All returns the list of resources.
func (s *Service) All() []resource.Name {
	return s.resources
}

// Add adds an additional resource to the list.
func (s *Service) Add(res resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := res.Validate(); err != nil {
		return errors.Errorf("unable to add resource: %s", err.Error())
	}

	idx := -1
	for i := range s.resources {
		if s.resources[i].UUID == res.UUID {
			idx = i
			break
		}
	}
	if idx != -1 {
		return errors.Errorf("resource with uuid %s already exists and cannot be added again", res.UUID)
	}

	s.resources = append(s.resources, res)
	return nil
}

// Replace replaces the resource list with another resource atomically.
func (s *Service) Replace(r []resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, res := range r {
		if err := res.Validate(); err != nil {
			return errors.Errorf("unable to replace resources: %s", err.Error())
		}
	}

	s.resources = r
	return nil
}

type ctxMetadataKey int

const (
	ctxKeyMetadata = ctxMetadataKey(iota)
)

// ContextWithService attaches a metadata Service to the given context.
func ContextWithService(ctx context.Context, s *Service) context.Context {
	return context.WithValue(ctx, ctxKeyMetadata, s)
}

// ContextService returns a metadata Service struct. It may be nil if the value was never set.
func ContextService(ctx context.Context) *Service {
	s := ctx.Value(ctxKeyMetadata)
	if s == nil {
		return nil
	}
	return s.(*Service)
}
