package main

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	pbgeneric "go.viam.com/api/component/generic/v1"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	myC "go.viam.com/rdk/examples/mycomponent/component"
	myCpb "go.viam.com/rdk/examples/mycomponent/proto/api/component/mycomponent/v1"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"

	goutils "go.viam.com/utils"
)

type myComponent struct {
	name string
	myString string
	myMotor motor.Motor
}

func (c *myComponent) GetName() string {
	return c.name
}

func (c *myComponent) GetString() string {
	return c.myString
}

func (c *myComponent) SetString(myString string) {
	c.myString = myString
}

func (c *myComponent) ZoomZoom(power float64) {
	c.myMotor.SetPower(context.Background(), power, nil)
}

type server struct {
	mu sync.Mutex
	components map[string]*myComponent
	mod *module.Module
	subServ subtype.Service
	pbgeneric.UnimplementedGenericServiceServer
}

func (s *server) AddComponent(ctx context.Context, cfg *config.Component, depList []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Debugf("Config: %+v", cfg)
	logger.Debugf("Deps: %+v", depList)

		logger.Debugf("SMURF200: %+v %+v", cfg.Model, myC.Model)

	if cfg.Model == myC.Model {
		c := myC.NewMyComponent(nil, *cfg, logger)
		m := make(map[resource.Name]interface{})
		m[resource.NameFromSubtype(myC.ResourceSubtype, cfg.Name)] = c
		err := s.subServ.Replace(m)
		if err != nil {
			return err
		}
		return nil
	}
	mName, err := resource.NewFromString("rdk:component:motor/" + cfg.Attributes.String("motor"))
	if err != nil {
		return err
	}

	r, err := s.mod.GetParentComponent(ctx, mName)
	if err != nil {
		return err
	}

	motor, ok := r.(motor.Motor)
	if !ok {
		return errors.Errorf("component (%s) is not a motor", mName)
	}
	s.components[cfg.Name] = &myComponent{name: cfg.Name, myMotor: motor}
	return nil
}

func (s *server) DoCommand(ctx context.Context, req *pbgeneric.DoCommandRequest) (*pbgeneric.DoCommandResponse, error) {
	cmd := req.Command.AsMap()
	c, ok := s.components[req.Name]
	if !ok {
		return nil, errors.Errorf("no component named: %s", req.Name)
	}

	returnVal := make(map[string]interface{})

	switch cmd["command"] {
	case "whoami":
		returnVal["name"] = c.GetName()
	case "getstring":
		returnVal["mystring"] = c.GetString()
	case "setstring":
		c.SetString(cmd["value"].(string))
	case "setspeed":
		c.ZoomZoom(cmd["speed"].(float64))
	}

	res, err := structpb.NewStruct(returnVal)
	if err != nil {
		return nil, err
	}

	return &pbgeneric.DoCommandResponse{Result: res}, nil
}


var logger = NewLogger()

func NewLogger() (*zap.SugaredLogger) {
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = []string{"/tmp/mod.log"}
	l, err := cfg.Build()
	if err != nil {
		return nil
	}
	return l.Sugar()
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	if len(args) < 2 {
		return errors.New("need socket path as command line argument")
	}
	socketPath := args[1]

	sub, err := subtype.New(make(map[resource.Name]interface{}))
	if err != nil {
		return err
	}
	server := &server{
		components: make(map[string]*myComponent),
		mod: module.NewModule(socketPath, logger),
		subServ: sub,
	}
	model, err := resource.NewModelFromString("acme:rocket:skates")
	if err != nil {
		return err
	}

	server.mod.RegisterModel(resource.RPCSubtype{
		Subtype: generic.Subtype,
		SvcName: pbgeneric.GenericService_ServiceDesc.ServiceName,
	}, model)
	server.mod.RegisterModel(resource.RPCSubtype{
		Subtype: myC.ResourceSubtype,
		SvcName: myCpb.MyComponentService_ServiceDesc.ServiceName,
	}, myC.Model)
	server.mod.RegisterAddComponent(server.AddComponent)
	pbgeneric.RegisterGenericServiceServer(server.mod.GRPCServer(), server)
	myCpb.RegisterMyComponentServiceServer(server.mod.GRPCServer(), myC.NewServer(server.subServ))

	err = server.mod.Start()
	defer server.mod.Close()
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
