// Package status implements a status service.
package status

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

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

// Status holds a resource name and it's corresponding status.
type Status struct {
	Name   resource.Name
	Status interface{}
}

// A Service returns statuses for resources when queried.
type Service interface {
	GetStatus(ctx context.Context, resourceNames []resource.Name) ([]Status, error)
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
	s := &statusService{
		resources: map[resource.Name]interface{}{},
		logger:    logger,
	}

	// trigger an update here
	resources := map[resource.Name]interface{}{}
	for _, n := range r.ResourceNames() {
		res, ok := r.ResourceByName(n)
		if !ok {
			return nil, utils.NewResourceNotFoundError(n)
		}
		resources[n] = res
	}
	if err := s.Update(ctx, resources); err != nil {
		return nil, err
	}
	return s, nil
}

type statusService struct {
	mu        sync.RWMutex
	resources map[resource.Name]interface{}
	logger    golog.Logger
}

// GetStatus takes a list of resource names and returns their corresponding statuses.
func (s *statusService) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]Status, error) {
	s.mu.RLock()
	// make a shallow copy of resources and then unlock
	resources := make(map[resource.Name]interface{}, len(s.resources))
	for name, resource := range s.resources {
		resources[name] = resource
	}
	s.mu.RUnlock()

	// dedupe resourceNames
	deduped := make(map[resource.Name]struct{}, len(resourceNames))
	for _, val := range resourceNames {
		deduped[val] = struct{}{}
	}
	statuses := make([]Status, 0, len(deduped))
	for name := range deduped {
		resource, ok := resources[name]
		if !ok {
			return nil, utils.NewResourceNotFoundError(name)
		}

		// if resource subtype has an associated CreateStatus method, use that
		// otherwise return true to indicate resource exists
		var status interface{} = true
		var err error
		subtype := registry.ResourceSubtypeLookup(name.Subtype)
		if subtype != nil && subtype.Status != nil {
			status, err = subtype.Status(ctx, resource)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get status from %q", name)
			}
		}
		statuses = append(statuses, Status{Name: name, Status: status})
	}
	return statuses, nil
}

// Update updates the status service when the robot has changed.
func (s *statusService) Update(ctx context.Context, r map[resource.Name]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources := map[resource.Name]interface{}{}
	for n, res := range r {
		resources[n] = res
	}
	s.resources = resources
	return nil
}
