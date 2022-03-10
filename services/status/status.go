// Package status implements a status service.
package status

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.StatusService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterStatusServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
}

// Status holds a resource name and its corresponding status. Status is expected to be comprised of string keys
// and values comprised of primitives, list of primitives, maps with string keys (or at least can be decomposed into one),
// or lists of the forementioned type of maps. Results with other types of data are not guaranteed.
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
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, err
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
	return s, nil
}

type statusService struct {
	mu        sync.RWMutex
	resources map[resource.Name]interface{}
	logger    golog.Logger
}

// GetStatus takes a list of resource names and returns their corresponding statuses. If no names are passed in, return all statuses.
func (s *statusService) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]Status, error) {
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

	statuses := make([]Status, 0, len(deduped))
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
