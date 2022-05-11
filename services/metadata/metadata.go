// Package metadata contains a service type that can be used to hold information
// about a robot's components and services.
package metadata

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/metadata/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
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
func New() (Service, error) {
	resources := []resource.Name{Name}
	return &metadataService{resources: resources}, nil
}

// metadataService keeps track of all resources associated with a robot.
type metadataService struct {
	mu        sync.Mutex
	resources []resource.Name
}

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.MetadataService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterMetadataServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New()
		},
	})
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
