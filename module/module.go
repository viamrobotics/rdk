// Package module provides services for external resource and logic modules.
package module

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/subtype"
)

// HandlerMap is the format for api->model pairs that the module will service.
// Ex: rdk:component:motor -> [acme:marine:thruster, acme:marine:outboard].
type HandlerMap map[resource.RPCSubtype][]resource.Model

// ToProto converts the HandlerMap to a protobuf representation.
func (h HandlerMap) ToProto() *pb.HandlerMap {
	pMap := &pb.HandlerMap{}
	for s, models := range h {
		subtype := &robotpb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				Subtype: s.Subtype,
				Name:    "",
			}),
			ProtoService: s.SvcName,
		}

		handler := &pb.HandlerDefinition{Subtype: subtype}
		for _, m := range models {
			handler.Models = append(handler.Models, m.String())
		}
		pMap.Handlers = append(pMap.Handlers, handler)
	}
	return pMap
}

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn *grpc.ClientConn) (HandlerMap, error) {
	hMap := make(HandlerMap)
	refClient := grpcreflect.NewClient(ctx, reflectpb.NewServerReflectionClient(conn))
	defer refClient.Reset()
	reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

	var errs error
	for _, h := range pMap.GetHandlers() {
		api := protoutils.ResourceNameFromProto(h.Subtype.Subtype).Subtype

		symDesc, err := reflSource.FindSymbol(h.Subtype.ProtoService)
		if err != nil {
			errs = multierr.Combine(errs, err)
			continue
		}
		svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
		if !ok {
			return nil, errors.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
		}
		subtype := &resource.RPCSubtype{
			Subtype: api,
			Desc:    svcDesc,
		}
		for _, m := range h.Models {
			model, err := resource.NewModelFromString(m)
			if err != nil {
				return nil, err
			}
			hMap[*subtype] = append(hMap[*subtype], model)
		}
	}
	return hMap, nil
}

// Module represents an external resource module that services components/services.
type Module struct {
	parent                  *client.RobotClient
	grpcServer              *grpc.Server
	logger                  *zap.SugaredLogger
	mu                      sync.Mutex
	ready                   bool
	addr                    string
	parentAddr              string
	activeBackgroundWorkers sync.WaitGroup
	handlers                HandlerMap
	services                map[resource.Subtype]subtype.Service
	pb.UnimplementedModuleServiceServer
}

// NewModule returns the basic module framework/structure.
func NewModule(address string, logger *zap.SugaredLogger) *Module {
	m := &Module{
		logger:     logger,
		addr:       address,
		grpcServer: grpc.NewServer(),
		ready:      true,
		handlers:   make(HandlerMap),
		services:   make(map[resource.Subtype]subtype.Service),
	}
	m.buildHandlerMap()
	pb.RegisterModuleServiceServer(m.grpcServer, m)
	reflection.Register(m.grpcServer)
	return m
}

// Start starts the module service and grpc server.
func (m *Module) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.initSubtypeServices(ctx)
	if err != nil {
		return err
	}

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

// GRPCServer returns the underlying grpc.Server instance for the module.
func (m *Module) GRPCServer() *grpc.Server {
	return m.grpcServer
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

// SetReady can be set to false if the module is not ready (ex. waiting on hardware).
func (m *Module) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = ready
}

// Ready receives the parent address and reports api/model combos the module is ready to service.
func (m *Module) Ready(ctx context.Context, req *pb.ReadyRequest) (*pb.ReadyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parentAddr = req.GetParentAddress()

	return &pb.ReadyResponse{
		Ready:      m.ready,
		Handlermap: m.handlers.ToProto(),
	}, nil
}

// AddComponent receives the component configuration from the parent.
func (m *Module) AddComponent(ctx context.Context, req *pb.AddComponentRequest) (*pb.AddComponentResponse, error) {
	cfg, err := config.ComponentConfigFromProto(req.Config)
	if err != nil {
		return nil, err
	}

	deps := make(registry.Dependencies)

	for _, c := range req.Dependencies {
		name, err := resource.NewFromString(c)
		if err != nil {
			return nil, err
		}
		c, err := m.GetParentComponent(ctx, name)
		if err != nil {
			return nil, err
		}
		deps[name] = c
	}

	creator := registry.ComponentLookup(cfg.ResourceName().Subtype, cfg.Model)
	if creator != nil && creator.Constructor != nil {
		comp, err := creator.Constructor(ctx, deps, *cfg, m.logger)
		if err != nil {
			return nil, err
		}

		wrapped := comp
		c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
		if c != nil && c.Reconfigurable != nil {
			wrapped, err = c.Reconfigurable(comp)
			if err != nil {
				return nil, multierr.Combine(err, utils.TryClose(ctx, comp))
			}
		}

		subSvc, ok := m.services[cfg.ResourceName().Subtype]
		if !ok {
			return nil, errors.Errorf("module can't service api: %s", cfg.ResourceName().Subtype)
		}

		err = subSvc.Add(cfg.ResourceName(), wrapped)
		if err != nil {
			return nil, err
		}
	}

	return &pb.AddComponentResponse{}, nil
}

func (m *Module) initSubtypeServices(ctx context.Context) error {
	for s, rs := range registry.RegisteredResourceSubtypes() {
		subSvc, ok := m.services[s]
		if !ok {
			newSvc, err := subtype.New(make(map[resource.Name]interface{}))
			if err != nil {
				return err
			}
			subSvc = newSvc
			m.services[s] = newSvc
		}

		if rs.RegisterRPCLiteService != nil {
			if err := rs.RegisterRPCLiteService(ctx, m.grpcServer, subSvc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Module) buildHandlerMap() {
	for c := range registry.RegisteredComponents() {
		split := strings.Split(c, "/")
		m.logger.Debugf("SMURF %+v", split)
		st, err := resource.NewSubtypeFromString(split[0])
		if err != nil {
			m.logger.Error(err)
			continue
		}
		model, err := resource.NewModelFromString(split[1])
		if err != nil {
			m.logger.Error(err)
			continue
		}
		creator := registry.ResourceSubtypeLookup(st)
		if creator.ReflectRPCServiceDesc == nil {
			m.logger.Errorf("rpc subtype %s doesn't contain a valid ReflectRPCServiceDesc", st)
			continue
		}
		rpcST := resource.RPCSubtype{
			Subtype: st,
			SvcName: creator.RPCServiceDesc.ServiceName,
			Desc:    creator.ReflectRPCServiceDesc,
		}
		m.handlers[rpcST] = append(m.handlers[rpcST], model)
	}
	m.logger.Debugf("SMURF %+v", m.handlers)
}
