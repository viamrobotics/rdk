// Package cloud implements a service to grab gRPC connections to talk to
// a cloud service that manages robots.
package cloud

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

// SubtypeName is a constant that identifies the internal cloud connection resource subtype string.
const SubtypeName = "cloud_connection"

// API is the fully qualified API for the internal cloud connection service.
var API = resource.APINamespaceRDKInternal.WithServiceType(SubtypeName)

// InternalServiceName is used to refer to/depend on this service internally.
var InternalServiceName = resource.NewName(API, "builtin")

// A ConnectionService supplies connections to a cloud service managing robots. Each
// connection should be closed when its not be used anymore.
type ConnectionService interface {
	resource.Resource
	AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error)
	AcquireConnectionAPIKey(ctx context.Context, apiKey, apiKeyID string) (string, rpc.ClientConn, error)
}

// NewCloudConnectionService makes a new cloud connection service to get gRPC connections
// to a cloud service managing robots.
func NewCloudConnectionService(cfg *config.Cloud, logger golog.Logger) ConnectionService {
	if cfg == nil || cfg.AppAddress == "" {
		return &cloudManagedService{
			Named: InternalServiceName.AsNamed(),
		}
	}
	return &cloudManagedService{
		Named:    InternalServiceName.AsNamed(),
		managed:  true,
		dialer:   rpc.NewCachedDialer(),
		cloudCfg: *cfg,
	}
}

type cloudManagedService struct {
	resource.Named
	// we assume the config is immutable for the lifetime of the process
	resource.TriviallyReconfigurable

	managed  bool
	cloudCfg config.Cloud
	logger   golog.Logger

	dialerMu sync.RWMutex
	dialer   rpc.Dialer
}

func (cm *cloudManagedService) AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error) {
	cm.dialerMu.RLock()
	defer cm.dialerMu.RUnlock()
	if !cm.managed {
		return "", nil, ErrNotCloudManaged
	}
	if cm.dialer == nil {
		return "", nil, errors.New("service closed")
	}

	ctx = rpc.ContextWithDialer(ctx, cm.dialer)
	timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := config.CreateNewGRPCClient(timeOutCtx, &cm.cloudCfg, cm.logger)
	return cm.cloudCfg.ID, conn, err
}

func (cm *cloudManagedService) AcquireConnectionAPIKey(ctx context.Context,
	apiKey, apiKeyID string,
) (string, rpc.ClientConn, error) {
	cm.dialerMu.RLock()
	defer cm.dialerMu.RUnlock()
	if !cm.managed {
		return "", nil, ErrNotCloudManaged
	}
	if cm.dialer == nil {
		return "", nil, errors.New("service closed")
	}

	ctx = rpc.ContextWithDialer(ctx, cm.dialer)
	timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := config.CreateNewGRPCClientWithAPIKey(timeOutCtx, &cm.cloudCfg, apiKey, apiKeyID, cm.logger)
	return cm.cloudCfg.ID, conn, err
}

func (cm *cloudManagedService) Close(ctx context.Context) error {
	cm.dialerMu.Lock()
	defer cm.dialerMu.Unlock()

	if cm.dialer != nil {
		utils.UncheckedError(cm.dialer.Close())
		cm.dialer = nil
	}

	return nil
}

// ErrNotCloudManaged is returned if a connection is requested but the robot is not
// yet cloud managed.
var ErrNotCloudManaged = errors.New("this robot is not cloud managed")
