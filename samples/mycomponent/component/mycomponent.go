// Package component implements MyComponent.
package component

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	pb "go.viam.com/rdk/samples/mycomponent/proto/api/component/mycomponent/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

var resourceSubtype = resource.NewSubtype(
	"acme",
	resource.ResourceTypeComponent,
	resource.SubtypeName("mycomponent"),
)

// Named is a helper for getting the named MyComponent's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(resourceSubtype, name)
}

func init() {
	registry.RegisterResourceSubtype(resourceSubtype, registry.ResourceSubtype{
		Reconfigurable: wrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.MyComponentService_ServiceDesc,
				newServer(subtypeSvc),
				pb.RegisterMyComponentServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.MyComponentService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return newClientFromConn(conn, name, logger)
		},
	})
	registry.RegisterComponent(resourceSubtype, "myActualComponent", registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newMyComponent(deps, config, logger), nil
		},
	})
}

type myActualComponent struct {
	deps   registry.Dependencies
	config config.Component
	logger golog.Logger
}

func newMyComponent(
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) MyComponent {
	return &myActualComponent{deps, config, logger}
}

func (mc *myActualComponent) DoOne(ctx context.Context, arg1 string) (bool, error) {
	return arg1 == "arg1", nil
}

func (mc *myActualComponent) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	if len(arg1) == 0 {
		return false, nil
	}
	ret := true
	for _, arg := range arg1 {
		ret = ret && arg == "arg1"
	}
	return ret, nil
}

func (mc *myActualComponent) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == "arg1", false, true, false}, nil
}

func (mc *myActualComponent) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	var rets []bool
	for _, arg := range arg1 {
		rets = append(rets, arg == "arg1")
	}
	return rets, nil
}

func (mc *myActualComponent) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}

// MyComponent is simple.
type MyComponent interface {
	DoOne(ctx context.Context, arg1 string) (bool, error)
	DoOneClientStream(ctx context.Context, arg1 []string) (bool, error)
	DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error)
	DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error)
	DoTwo(ctx context.Context, arg1 bool) (string, error)
}

func wrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	mc, ok := r.(MyComponent)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("MyComponent", r)
	}
	if reconfigurable, ok := mc.(*reconfigurableMyComponent); ok {
		return reconfigurable, nil
	}
	return &reconfigurableMyComponent{actual: mc}, nil
}

var (
	_ = MyComponent(&reconfigurableMyComponent{})
	_ = resource.Reconfigurable(&reconfigurableMyComponent{})
)

type reconfigurableMyComponent struct {
	mu     sync.RWMutex
	actual MyComponent
}

func (mc *reconfigurableMyComponent) ProxyFor() interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual
}

func (mc *reconfigurableMyComponent) DoOne(ctx context.Context, arg1 string) (bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual.DoOne(ctx, arg1)
}

func (mc *reconfigurableMyComponent) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual.DoOneClientStream(ctx, arg1)
}

func (mc *reconfigurableMyComponent) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual.DoOneServerStream(ctx, arg1)
}

func (mc *reconfigurableMyComponent) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual.DoOneBiDiStream(ctx, arg1)
}

func (mc *reconfigurableMyComponent) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.actual.DoTwo(ctx, arg1)
}

func (mc *reconfigurableMyComponent) Reconfigure(ctx context.Context, newMyComponenet resource.Reconfigurable) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	actual, ok := newMyComponenet.(*reconfigurableMyComponent)
	if !ok {
		return utils.NewUnexpectedTypeError(mc, newMyComponenet)
	}
	if err := goutils.TryClose(ctx, mc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	mc.actual = actual.actual
	return nil
}

// subtypeServer implements the MyComponent from gripper.proto.
type subtypeServer struct {
	pb.UnimplementedMyComponentServiceServer
	s subtype.Service
}

func newServer(s subtype.Service) pb.MyComponentServiceServer {
	return &subtypeServer{s: s}
}

func (s *subtypeServer) getMyComponent(name string) (MyComponent, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no MyComponent with name (%s)", name)
	}
	mc, ok := resource.(MyComponent)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a MyComponent", name)
	}
	return mc, nil
}

func (s *subtypeServer) DoOne(ctx context.Context, req *pb.DoOneRequest) (*pb.DoOneResponse, error) {
	mc, err := s.getMyComponent(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := mc.DoOne(ctx, req.Arg1)
	if err != nil {
		return nil, err
	}
	return &pb.DoOneResponse{Ret1: resp}, nil
}

func (s *subtypeServer) DoOneClientStream(server pb.MyComponentService_DoOneClientStreamServer) error {
	var name string
	var args []string
	for {
		msg, err := server.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		args = append(args, msg.Arg1)
		if name == "" {
			name = msg.Name
			continue
		}
		if name != msg.Name {
			return errors.New("unexpected")
		}
	}
	mc, err := s.getMyComponent(name)
	if err != nil {
		return err
	}
	resp, err := mc.DoOneClientStream(server.Context(), args)
	if err != nil {
		return err
	}
	return server.SendAndClose(&pb.DoOneClientStreamResponse{Ret1: resp})
}

func (s *subtypeServer) DoOneServerStream(req *pb.DoOneServerStreamRequest, stream pb.MyComponentService_DoOneServerStreamServer) error {
	mc, err := s.getMyComponent(req.Name)
	if err != nil {
		return err
	}
	resp, err := mc.DoOneServerStream(stream.Context(), req.Arg1)
	if err != nil {
		return err
	}
	for _, ret := range resp {
		if err := stream.Send(&pb.DoOneServerStreamResponse{
			Ret1: ret,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *subtypeServer) DoOneBiDiStream(server pb.MyComponentService_DoOneBiDiStreamServer) error {
	var name string
	var args []string
	for {
		msg, err := server.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		args = append(args, msg.Arg1)
		if name == "" {
			name = msg.Name
			continue
		}
		if name != msg.Name {
			return errors.New("unexpected")
		}
	}
	mc, err := s.getMyComponent(name)
	if err != nil {
		return err
	}
	resp, err := mc.DoOneBiDiStream(server.Context(), args)
	if err != nil {
		return err
	}
	for _, respRet := range resp {
		if err := server.Send(&pb.DoOneBiDiStreamResponse{Ret1: respRet}); err != nil {
			return err
		}
	}
	return nil
}

func (s *subtypeServer) DoTwo(ctx context.Context, req *pb.DoTwoRequest) (*pb.DoTwoResponse, error) {
	mc, err := s.getMyComponent(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := mc.DoTwo(ctx, req.Arg1)
	if err != nil {
		return nil, err
	}
	return &pb.DoTwoResponse{Ret1: resp}, nil
}

func newClientFromConn(conn rpc.ClientConn, name string, logger golog.Logger) MyComponent {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewMyComponentServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

type serviceClient struct {
	conn   rpc.ClientConn
	client pb.MyComponentServiceClient
	logger golog.Logger
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an gripper client.
type client struct {
	*serviceClient
	name string
}

func clientFromSvcClient(sc *serviceClient, name string) MyComponent {
	return &client{sc, name}
}

func (c *client) DoOne(ctx context.Context, arg1 string) (bool, error) {
	resp, err := c.client.DoOne(ctx, &pb.DoOneRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return false, err
	}
	return resp.Ret1, nil
}

func (c *client) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	client, err := c.client.DoOneClientStream(ctx)
	if err != nil {
		return false, err
	}
	for _, arg := range arg1 {
		if err := client.Send(&pb.DoOneClientStreamRequest{
			Name: c.name,
			Arg1: arg,
		}); err != nil {
			return false, err
		}
	}
	resp, err := client.CloseAndRecv()
	if err != nil {
		return false, err
	}
	return resp.Ret1, nil
}

func (c *client) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	resp, err := c.client.DoOneServerStream(ctx, &pb.DoOneServerStreamRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return nil, err
	}
	var rets []bool
	for {
		resp, err := resp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		rets = append(rets, resp.Ret1)
	}
	return rets, nil
}

func (c *client) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	client, err := c.client.DoOneBiDiStream(ctx)
	if err != nil {
		return nil, err
	}
	for _, arg := range arg1 {
		if err := client.Send(&pb.DoOneBiDiStreamRequest{
			Name: c.name,
			Arg1: arg,
		}); err != nil {
			return nil, err
		}
	}
	if err := client.CloseSend(); err != nil {
		return nil, err
	}

	var rets []bool
	for {
		resp, err := client.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		rets = append(rets, resp.Ret1)
	}
	return rets, nil
}

func (c *client) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	resp, err := c.client.DoTwo(ctx, &pb.DoTwoRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return "", err
	}
	return resp.Ret1, nil
}

func (c *client) Close() error {
	return c.serviceClient.Close()
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
