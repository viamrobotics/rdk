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
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
)

type (
	AddComponentFunc func(ctx context.Context, cfg *config.Component, depList []string) error
)

type HandlerMap map[resource.Subtype][]resource.Model

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

func (s *modserver) Ready(ctx context.Context, req *pb.ReadyRequest) (*pb.ReadyResponse, error) {
	return &pb.ReadyResponse{Ready: s.module.ready, Handlermap: s.module.handlers.ToProto()}, nil
}

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

type Module struct {
	name                    string
	parent                  *client.RobotClient
	modServer               *modserver
	grpcServer              *grpc.Server
	logger                  *zap.SugaredLogger
	mu                      sync.Mutex
	ready                   bool
	addr                    string
	activeBackgroundWorkers sync.WaitGroup
	addComponent            AddComponentFunc
	handlers                HandlerMap
}

func (m *Module) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
}

func (m *Module) GRPCServer() *grpc.Server {
	return m.grpcServer
}

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
		defer os.Remove(m.addr)
		m.logger.Debugf("server listening at %v", lis.Addr())
		if err := m.grpcServer.Serve(lis); err != nil {
			m.logger.Fatalf("failed to serve: %v", err)
		}
	})
	return nil
}

func (m *Module) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Debug("Sutting down gracefully.")
	m.grpcServer.GracefulStop()
	m.activeBackgroundWorkers.Wait()
}

func (m *Module) RegisterAddComponent(componentFunc AddComponentFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addComponent = componentFunc
}

func (m *Module) RegisterModel(api resource.Subtype, model resource.Model) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[api] = append(m.handlers[api], model)
}

func (m *Module) GetParentComponent(name resource.Name) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.parent == nil {
		rc, err := client.New(context.Background(), "localhost:8080", m.logger)
		if err != nil {
			return nil, err
		}
		m.parent = rc
	}
	return m.parent.ResourceByName(name)
}

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
