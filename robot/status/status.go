// Package status implements a status service.
package status

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// A Service returns statuses for resources when queried.
type Service interface {
	GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error)
}

// New returns a new status service for the given robot.
func New(ctx context.Context, r robot.Robot, logger golog.Logger) Service {
	s := &statusService{
		resources: map[resource.Name]interface{}{},
		logger:    logger,
	}
	return s
}

type statusService struct {
	mu        sync.RWMutex
	resources map[resource.Name]interface{}
	logger    golog.Logger
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (s *statusService) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	s.mu.RLock()
	// make a shallow copy of resources and then unlock
	resources := make(map[resource.Name]interface{}, len(s.resources))
	for name, resource := range s.resources {
		resources[name] = resource
	}
	s.mu.RUnlock()

	namesToDedupe := resourceNames
	// if no names, return all
	if len(namesToDedupe) == 0 {
		namesToDedupe = make([]resource.Name, 0, len(resources))
		for n := range resources {
			namesToDedupe = append(namesToDedupe, n)
		}
	}

	// dedupe resourceNames
	deduped := make(map[resource.Name]struct{}, len(namesToDedupe))
	for _, val := range namesToDedupe {
		deduped[val] = struct{}{}
	}

	statuses := make([]robot.Status, 0, len(deduped))
	for name := range deduped {
		resource, ok := resources[name]
		if !ok {
			return nil, utils.NewResourceNotFoundError(name)
		}

		// if resource subtype has an associated CreateStatus method, use that
		// otherwise return true to indicate resource exists
		var status interface{} = struct{}{}
		var err error
		subtype := registry.ResourceSubtypeLookup(name.Subtype)
		if subtype != nil && subtype.Status != nil {
			status, err = subtype.Status(ctx, resource)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get status from %q", name)
			}
		}
		statuses = append(statuses, robot.Status{Name: name, Status: status})
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
