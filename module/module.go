// Package module provides services for external resource and logic modules.
package module

import (
	"context"
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
)

type (
	// AddComponentFunc is the signature of a function to register that will handle new configs.
	AddComponentFunc func(ctx context.Context, cfg *config.Component, depList []string) error

	// HandlerMap is the format for api->model pairs that the module will service.
	HandlerMap map[resource.Subtype][]resource.Model
)

// ToProto converts the HandlerMap to a protobuf reprsentation.
func (h HandlerMap) ToProto() *pb.HandlerMap {
	pMap := &pb.HandlerMap{}
	for api, models := range h {
		handler := &pb.HandlerDefinition{Api: api.String()}
		for _, m := range models {
			handler.Models = append(handler.Models, m.String())
		}
		pMap.Handlers = append(pMap.Handlers, handler)
	}
	return pMap
}

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(pMap *pb.HandlerMap) (HandlerMap, error) {
	hMap := make(HandlerMap)
	for _, h := range pMap.GetHandlers() {
		api, err := resource.NewSubtypeFromString(h.Api)
		if err != nil {
			return nil, err
		}
		for _, m := range h.Models {
			model, err := resource.NewModelFromString(m)
			if err != nil {
				return nil, err
			}
			hMap[api] = append(hMap[api], model)
		}
	}
	return hMap, nil
}

type modserver struct {
	module *Module
	pb.UnimplementedModuleServiceServer
}

// Ready receives the parent address and reports api/model combos the module is ready to service.
func (s *modserver) Ready(ctx context.Context, req *pb.ReadyRequest) (*pb.ReadyResponse, error) {
	s.module.mu.Lock()
	defer s.module.mu.Unlock()
	s.module.parentAddr = req.GetParentAddress()
	return &pb.ReadyResponse{Ready: s.module.ready, Handlermap: s.module.handlers.ToProto()}, nil
}

// AddComponent receives the component configuration from the parent.
func (s *modserver) AddComponent(ctx context.Context, req *pb.AddComponentRequest) (*pb.AddComponentResponse, error) {
	if s.module.addComponent == nil {
		return nil, errors.WithStack(errors.New("no AddComponentFunc registered"))
	}

	cfg, err := config.ComponentConfigFromProto(req.Config)
	if err != nil {
		return nil, err
	}
	return &pb.AddComponentResponse{}, s.module.addComponent(ctx, cfg, req.Dependencies)
}

// Module represents an external resource module that services components/services.
type Module struct {
	parent                  *client.RobotClient
	modServer               *modserver
	grpcServer              *grpc.Server
	logger                  *zap.SugaredLogger
	mu                      sync.Mutex
	ready                   bool
	addr                    string
	parentAddr              string
	activeBackgroundWorkers sync.WaitGroup
	addComponent            AddComponentFunc
	handlers                HandlerMap
}

// SetReady can be set to false if the module is not ready (ex. waiting on hardware).
func (m *Module) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
}

// GRPCServer returns the underlying grpc.Server instance for the module.
func (m *Module) GRPCServer() *grpc.Server {
	return m.grpcServer
}

// Start starts the module service and grpc server.
func (m *Module) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldMask := syscall.Umask(0o077)
	lis, err := net.Listen("unix", m.addr)
	syscall.Umask(oldMask)
	if err != nil {
		return errors.WithMessage(err, "failed to listen")
	}

	m.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer m.activeBackgroundWorkers.Done()
		defer utils.UncheckedErrorFunc(func() error { return os.Remove(m.addr) })
		m.logger.Infof("server listening at %v", lis.Addr())
		if err := m.grpcServer.Serve(lis); err != nil {
			m.logger.Fatalf("failed to serve: %v", err)
		}
	})
	return nil
}

// Close shuts down the module and grpc server.
func (m *Module) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Info("Sutting down gracefully.")
	m.grpcServer.GracefulStop()
	m.activeBackgroundWorkers.Wait()
}

// RegisterAddComponent allows a module to register a function to be called when the parent
// asks it to handle a new component.
func (m *Module) RegisterAddComponent(componentFunc AddComponentFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addComponent = componentFunc
}

// RegisterModel registers the list of api/model pairs this module will service.
func (m *Module) RegisterModel(api resource.Subtype, model resource.Model) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[api] = append(m.handlers[api], model)
}

// GetParentComponent returns a component from the parent robot by name.
func (m *Module) GetParentComponent(ctx context.Context, name resource.Name) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Infof("Address: %s", m.parentAddr)
	if m.parent == nil {
		rc, err := client.New(ctx, "unix://"+m.parentAddr, m.logger,
			client.WithDialOptions(rpc.WithForceDirectGRPC(), rpc.WithInsecure()))
		if err != nil {
			return nil, err
		}
		m.parent = rc
	}
	return m.parent.ResourceByName(name)
}

// NewModule returns the basic module framework/structure.
func NewModule(address string, logger *zap.SugaredLogger) *Module {
	m := &Module{
		logger:     logger,
		addr:       address,
		grpcServer: grpc.NewServer(),
		ready:      true,
		handlers:   make(HandlerMap),
	}

	m.modServer = &modserver{module: m}

	pb.RegisterModuleServiceServer(m.grpcServer, m.modServer)
	return m
}
