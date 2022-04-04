// Package configuration discovers components and potential configurations
package configuration

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/configuration/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.ConfigurationService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterConfigurationServiceHandlerFromEndpoint,
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
	},
	)
	cType := config.ServiceType(SubtypeName)
	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &Config{})
}

// A Service controls the configuration for a robot.
type Service interface {
	GetCameras(ctx context.Context) ([]string, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("configuration")

// Subtype is a constant that identifies the configuration service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the ConfigurationService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// Config describes how to configure the service.
type Config struct {
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	return nil
}

// New returns a new configuration service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	configSvc := &configService{
		r:          r,
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return configSvc, nil
}

type configService struct {
	mu sync.RWMutex
	r  robot.Robot

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *configService) GetCameras(ctx context.Context) ([]string, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return []string{"foo"}, nil
}
