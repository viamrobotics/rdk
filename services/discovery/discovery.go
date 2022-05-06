// Package discovery implements a discovery service.
package discovery

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/discovery/v1"
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
				&pb.DiscoveryService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterDiscoveryServiceHandlerFromEndpoint,
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

// Discovery holds a resource name and its corresponding discovery. Discovery is expected
// to be comprised of string keys and values comprised of primitives, list of primitives,
// maps with string keys (or at least can be decomposed into one), or lists of the
// forementioned type of maps. Results with other types of data are not guaranteed.
type Discovery struct {
	Name       resource.Name
	Discovered interface{}
}

// A Service returns discoveries for resources when queried.
type Service interface {
	Discover(ctx context.Context, resourceNames []resource.Name) ([]Discovery, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("discovery")

// Subtype is a constant that identifies the discovery service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the DiscoveryService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the discovery service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("discovery.Service", resource)
	}
	return svc, nil
}

// New returns a new discovery service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	s := &discoveryService{
		resources: map[resource.Name]interface{}{},
		logger:    logger,
	}
	return s, nil
}

type discoveryService struct {
	mu        sync.RWMutex
	resources map[resource.Name]interface{}
	logger    golog.Logger
}

// Discover takes a list of resource names and returns their corresponding discoveries.
// If no names are passed in, return all discoveries.
func (s *discoveryService) Discover(ctx context.Context, resourceNames []resource.Name) ([]Discovery, error) {
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

	discoveries := make([]Discovery, 0, len(deduped))
	for name := range deduped {
		resource, ok := resources[name]
		if !ok {
			return nil, utils.NewResourceNotFoundError(name)
		}

		// if resource subtype has an associated Discover method, use that
		// otherwise return true to indicate resource exists
		var discovery interface{} = struct{}{}
		var err error
		subtype := registry.ResourceSubtypeLookup(name.Subtype)
		if subtype != nil && subtype.Discovered != nil {
			discovery, err = subtype.Discovered(ctx, resource)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get discovery from %q", name)
			}
		}
		discoveries = append(discoveries, Discovery{Name: name, Discovered: discovery})
	}
	return discoveries, nil
}

// Update updates the discovery service when the robot has changed.
func (s *discoveryService) Update(ctx context.Context, r map[resource.Name]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources := map[resource.Name]interface{}{}
	for n, res := range r {
		resources[n] = res
	}
	s.resources = resources
	return nil
}
