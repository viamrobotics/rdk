// Package status implements a status service.
package status

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// ResourceStatus holds a resource name and it's corresponding status.
type ResourceStatus struct {
	Name   resource.Name
	Status interface{}
}

// A Service returns statuses for resources when queried.
type Service interface {
	GetStatus(ctx context.Context, resourceNames []resource.Name) ([]ResourceStatus, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("status")

// Subtype is a constant that identifies the status service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the StatusService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the status service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, ok := r.ResourceByName(Name)
	if !ok {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("status.Service", resource)
	}
	return svc, nil
}

// New returns a new status service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	return &statusService{
		r:      r,
		logger: logger,
	}, nil
}

type statusService struct {
	r      robot.Robot
	logger golog.Logger
}

// GetStatus takes a list of resource names and returns their corresponding statuses.
func (s statusService) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]ResourceStatus, error) {
	return []ResourceStatus{}, nil
}
