// Package metadata contains a service type that can be used to hold information
// about a robot's components and services.
package metadata

import (
	"context"
	"sync"

	"go.viam.com/rdk/resource"
)

// Service defines what a metadata service should be able to do.
type Service interface {
	// Resources returns the list of resources.
	Resources(ctx context.Context) ([]resource.Name, error)
}

// New creates a new Service struct and initializes the resource list with a metadata service.
func New() Service {
	resources := []resource.Name{}
	return &metadataService{resources: resources}
}

// metadataService keeps track of all resources associated with a robot.
type metadataService struct {
	mu        sync.Mutex
	resources []resource.Name
}

// Resources returns the list of resources.
func (svc *metadataService) Resources(ctx context.Context) ([]resource.Name, error) {
	resources := []resource.Name{}

	resources = append(resources, svc.resources...)
	return resources, nil
}

// Update updates metadata service using the currently registered parts of the robot.
func (svc *metadataService) Update(ctx context.Context, curResources map[resource.Name]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	resources := []resource.Name{}
	for name := range curResources {
		resources = append(resources, name)
	}

	svc.resources = resources
	return nil
}
