// Package metadata contains a service type that can be used to hold information
// about a robot's components and services.
// CR erodkin: should this just be part of the robot package? probably
package metadata

import (
	"context"
	"sync"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// SubtypeName is a constant that identifies the component resource subtype.
const SubtypeName = resource.SubtypeName("metadata")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the StatusService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot receives the metadata service of a robot.
func FromRobot(robot robot.Robot) (Service, error) {
	resource, err := robot.ResourceByName(Name)
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("metadata.Service", resource)
	}
	return svc, nil
}

// Service defines what a metadata service should be able to do.
type Service interface {
	// Resources returns the list of resources.
	Resources(ctx context.Context) ([]resource.Name, error)
}

// New creates a new Service struct and initializes the resource list with a metadata service.
func New() Service {
	resources := []resource.Name{Name}
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
