// Package resources contains a Metadata type that can be used to hold information about a robot's components and services.
package resources

import (
	"sync"

	"github.com/go-errors/errors"

	"github.com/google/uuid"
	pb "go.viam.com/core/proto/api/service/v1"
)

// Define a few known constants
const (
	ResourceNamespaceCore   = "core"
	ResourceTypeService     = "service"
	ResourceSubtypeMetadata = "metadata"
)

// validateResourceName ensures that important fields exist and are valid
func validateResourceName(resource *pb.ResourceName) error {
	if _, err := uuid.Parse(resource.Uuid); err != nil {
		return errors.New("uuid field for resource missing or invalid.")
	}
	if resource.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid.")
	}
	if resource.Type == "" {
		return errors.New("type field for resource missing or invalid.")
	}
	if resource.Subtype == "" {
		return errors.New("subtype field for resource missing or invalid.")
	}
	return nil
}

type Resources struct {
	mu        sync.Mutex
	resources []pb.ResourceName
}

// New creates a new Resources struct and initializes the resource list with a metadata service.
func New() Resources {
	resources := []pb.ResourceName{
		{
			Uuid:      uuid.NewString(),
			Namespace: ResourceNamespaceCore,
			Type:      ResourceTypeService,
			Subtype:   ResourceSubtypeMetadata,
			Name:      "",
		},
	}

	return Resources{resources: resources}
}

// Resources returns the list of resources.
func (r *Resources) Resources() []pb.ResourceName {
	return r.resources
}

// AddResource adds an additional resource to the list. Cannot add another metadata service
func (r *Resources) AddResource(resource *pb.ResourceName) error {
	if err := validateResourceName(resource); err != nil {
		return errors.Errorf("Unable to add resource: %s", err.Error())
	}
	if resource.Subtype == ResourceSubtypeMetadata {
		return errors.New("Unable to add a resource with a metadata subtype.")
	}

	idx := -1
	for i := range r.resources {
		if r.resources[i].Uuid == resource.Uuid {
			idx = i
			break
		}
	}
	if idx != -1 {
		return errors.Errorf("Resource with uuid %s already exists and cannot be added again.", resource.Uuid)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resources = append(
		r.resources,
		pb.ResourceName{
			Uuid:      resource.Uuid,
			Namespace: resource.Namespace,
			Type:      resource.Type,
			Subtype:   resource.Subtype,
			Name:      resource.Name,
		},
	)
	return nil
}

// RemoveResource removes resource from the list.
func (r *Resources) RemoveResource(resource *pb.ResourceName) error {
	if err := validateResourceName(resource); err != nil {
		return errors.Errorf("Invalid resource to search for: %s", err.Error())
	}
	if resource.Subtype == ResourceSubtypeMetadata {
		return errors.New("Unable to remove resource with a metadata subtype.")
	}
	idx := -1
	for i := range r.resources {
		if r.resources[i].Uuid == resource.Uuid {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.Errorf("Unable to find resource with uuid %s.", resource.Uuid)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resources = append(r.resources[:idx], r.resources[idx+1:]...)
	return nil
}

// ClearResources clears all resources except the metadata service from the resource list
func (r *Resources) ClearResources() error {
	idx := -1
	for i := range r.resources {
		if r.resources[i].Subtype == ResourceSubtypeMetadata {
			idx = i
			break
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if idx != -1 {
		r.resources = []pb.ResourceName{
			{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeService,
				Subtype:   ResourceSubtypeMetadata,
				Name:      "",
			},
		}
	} else {
		var newList []pb.ResourceName
		copy(newList, r.resources[idx:idx+1])
		r.resources = newList
	}
	return nil
}
