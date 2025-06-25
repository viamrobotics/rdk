// Package discovery implements the discovery service, which lets users surface resource configs for their machines to use.
// For more information, see the [Discovery service docs].
//
// [Discovery service docs]: https://docs.viam.com/dev/reference/apis/services/discovery/
package discovery

import (
	"context"
	"errors"

	pb "go.viam.com/api/service/discovery/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterDiscoveryServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.DiscoveryService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is the name of the type of service.
const (
	SubtypeName = "discovery"
)

// API is a variable that identifies the discovery resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// ErrNilResponse is the error for when a nil response is returned from a discovery service.
var ErrNilResponse = errors.New("discovery service returned a nil response")

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named discovery service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromDependencies is a helper for getting the named discovery service from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// Service describes the functions that are available to the service.
//
// For more information, see the [Discovery service docs].
//
// DiscoverResources example:
//
//		// Get the discovered resources of a Discovery Service.
//		cfgs, err := myDiscoveryService.DiscoverResources(ctx, nil)
//		if err != nil {
//			logger.Fatal(err)
//		}
//	 	// Print out the discovered resources.
//		for _, cfg := range cfgs {
//			fmt.Printf("Name: %v\tModel: %v\tAPI: %v", cfg.Name, cfg.Model, cfg.API)
//			fmt.Printf("Attributes: ", cfg.Attributes)
//		}
//
// For more information, see the [discover resources method docs].
//
// [Discovery service docs]: https://docs.viam.com/dev/reference/apis/services/discovery/
// [discover resources method docs]: https://docs.viam.com/dev/reference/apis/services/discovery/#discoverresources
type Service interface {
	resource.Resource
	DiscoverResources(ctx context.Context, extra map[string]any) ([]resource.Config, error)
}
