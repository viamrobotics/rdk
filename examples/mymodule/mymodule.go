package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module"
	pbgeneric "go.viam.com/rdk/proto/api/component/generic/v1"
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
	pbgeneric.UnimplementedGenericServiceServer
}

func (s *server) AddComponent(ctx context.Context, cfg *config.Component, depList []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Debugf("Config: %+v", cfg)
	logger.Debugf("Deps: %+v", depList)

	motorname, ok := cfg.Attributes["motor"]
	if !ok || motorname == "" {
		return errors.New("motor_name must be set in the config")
	}

	r, err := s.mod.GetParentComponent(motorname.(string))
	if err != nil {
		return err
	}

	motor, ok := r.(motor.Motor)
	if !ok {
		return errors.Errorf("component %s is not a motor", motorname)
	}
	s.components[cfg.Name] = &myComponent{name: cfg.Name, myMotor: motor}
	return nil
}

func (s *server) Do(ctx context.Context, req *pbgeneric.DoRequest) (*pbgeneric.DoResponse, error) {
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

	return &pbgeneric.DoResponse{Result: res}, nil
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
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	signal.Notify(shutdown, syscall.SIGTERM)

	server := &server{components: make(map[string]*myComponent)}

	server.mod = module.NewModule(os.Args[1], logger)
	server.mod.RegisterAddComponent(server.AddComponent)
	pbgeneric.RegisterGenericServiceServer(server.mod.GRPCServer(), server)

	err := server.mod.Start()
	defer server.mod.Close()
	if err != nil {
		logger.Error(err)
		return
	}
	<-shutdown
}
