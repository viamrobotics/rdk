package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/baseremotecontrol"
)

// BaseRemoteControlService represents a fake instance of a base remote control service.
type BaseRemoteControlService struct {
	baseremotecontrol.Service
	name          resource.Name
	DoCommandFunc func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewBaseRemoteControlService returns a new injected base remote control service.
func NewBaseRemoteControlService(name string) *BaseRemoteControlService {
	return &BaseRemoteControlService{name: baseremotecontrol.Named(name)}
}

// Name returns the name of the resource.
func (ns *BaseRemoteControlService) Name() resource.Name {
	return ns.name
}

// DoCommand calls the injected DoCommand or the real variant.
func (ns *BaseRemoteControlService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if ns.DoCommandFunc == nil {
		return ns.Service.DoCommand(ctx, cmd)
	}
	return ns.DoCommandFunc(ctx, cmd)
}
