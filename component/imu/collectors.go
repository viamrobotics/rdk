package imu

import (
	"context"
	"github.com/golang/protobuf/ptypes/any"
	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"time"
)

// TODO: parameters

type ReadAngularVelocityCapturer struct {
	client pb.IMUServiceClient
}

type ReadOrientationCapturer struct {
	client pb.IMUServiceClient
}

type ReadAccelerationCapturer struct {
	client pb.IMUServiceClient
}

func (c ReadAngularVelocityCapturer) Capture(name string) (*any.Any, error) {
	req := pb.ReadAngularVelocityRequest{Name: name}
	// TODO: what should context be here?
	resp, err := c.client.ReadAngularVelocity(context.TODO(), &req)
	if err != nil {
		return nil, err
	}
	a, err := anypb.New(resp)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (c ReadOrientationCapturer) Capture(name string) (*any.Any, error) {
	req := pb.ReadOrientationRequest{Name: name}
	// TODO: what should context be here?
	resp, err := c.client.ReadOrientation(context.TODO(), &req)
	if err != nil {
		return nil, err
	}
	a, err := anypb.New(resp)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (c ReadAccelerationCapturer) Capture(name string) (*any.Any, error) {
	req := pb.ReadAccelerationRequest{Name: name}
	// TODO: what should context be here?
	resp, err := c.client.ReadAcceleration(context.TODO(), &req)
	if err != nil {
		return nil, err
	}
	a, err := anypb.New(resp)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func NewReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) data.Collector {
	c := ReadAngularVelocityCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, name, target)
}

func NewReadOrientationCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) data.Collector {
	c := ReadOrientationCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, name, target)
}

func NewReadAccelerationCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) data.Collector {
	c := ReadAccelerationCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, name, target)
}
