// Package discovery implements a discovery service.
package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
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
	Key        Key
	Discovered interface{}
}

// A Service returns discoveries for resources when queried.
type Service interface {
	Discover(ctx context.Context, keys []Key) ([]Discovery, error)
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

type (
	discoveryService struct {
		mu        sync.RWMutex
		resources map[resource.Name]interface{}
		logger    golog.Logger
	}

	Key struct {
		subtypeName resource.SubtypeName
		model       string
	}
)

// Discover takes a list of subtype and model name pairs and returns their corresponding
// discoveries.
func (s *discoveryService) Discover(ctx context.Context, keys []Key) ([]Discovery, error) {
	// dedupe keys
	deduped := make(map[Key]struct{}, len(keys))
	for _, k := range keys {
		deduped[k] = struct{}{}
	}

	discoveries := make([]Discovery, 0, len(deduped))
	for key := range deduped {
		discoveryFunction, ok := DiscoveryFunctionLookup(key.subtypeName, key.model)
		if !ok {
			return nil, fmt.Errorf("no discovery function registered for %q and model %q.", key.subtypeName, key.model)
		}

		if discoveryFunction != nil {
			discovery, err := discoveryFunction(ctx, key.subtypeName, key.model)
			if err != nil {
				return nil, fmt.Errorf("failed to get discovery for subtype %q and model %q", key.subtypeName, key.model)
			}
			discoveries = append(discoveries, Discovery{Key: key, Discovered: discovery})
		}
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
