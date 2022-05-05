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

// Discovery holds a subtype name and its corresponding discovery. Discovery is expected
// to be comprised of string keys and values comprised of primitives, list of primitives,
// maps with string keys (or at least can be decomposed into one), or lists of the
// forementioned type of maps. Results with other types of data are not guaranteed.
type Discovery struct {
	Name       resource.SubtypeName
	Discovered interface{}
}

// A Service returns discoveries for subtype when queried.
type Service interface {
	Discover(ctx context.Context, subtypeNames []resource.SubtypeName) ([]Discovery, error)
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
		subtypes: map[resource.SubtypeName]interface{}{},
		logger:   logger,
	}
	return s, nil
}

type discoveryService struct {
	mu       sync.RWMutex
	subtypes map[resource.SubtypeName]interface{}
	logger   golog.Logger
}

// Discover takes a list of subtype names and returns their corresponding discoveries.
func (s *discoveryService) Discover(ctx context.Context, subtypeNames []resource.SubtypeName) ([]Discovery, error) {
	// get subtypes by name
	subtypesByName := map[resource.SubtypeName]*registry.ResourceSubtype{}
	for subtype, registration := range registry.RegisteredResourceSubtypes() {
		subtypesByName[subtype.ResourceSubtype] = &registration
	}

	// dedupe subtypeNames
	deduped := make(map[resource.SubtypeName]struct{}, len(subtypeNames))
	for _, name := range subtypeNames {
		deduped[name] = struct{}{}
	}

	discoveries := make([]Discovery, 0, len(deduped))
	for name := range deduped {
		// if subtype has an associated Discover method, use that
		// otherwise return true to indicate resource exists
		var discovery interface{} = struct{}{}
		var err error
		registration, ok := subtypesByName[name]
		if ok && registration != nil && registration.Discovered != nil {
			discovery, err = registration.Discovered(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get discovery from %q", name)
			}
		}
		discoveries = append(discoveries, Discovery{Name: name, Discovered: discovery})
	}
	return discoveries, nil
}
