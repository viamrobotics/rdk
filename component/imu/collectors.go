package imu

import (
	"context"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/utils/rpc"
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

func (c ReadAngularVelocityCapturer) Capture(params map[string]string) (*any.Any, error) {
	name, ok := params["name"]
	if !ok {
		return nil, errors.New("must pass name parameter to ReadAngularVelocity")
	}

	req := pb.ReadAngularVelocityRequest{
		Name: name,
	}
	// TODO: what should context be here?
	return data.WrapProtoAll(c.client.ReadAngularVelocity(context.TODO(), &req))
}

func (c ReadOrientationCapturer) Capture(params map[string]string) (*any.Any, error) {
	name, ok := params["name"]
	if !ok {
		return nil, errors.New("must pass name parameter to ReadOrientation")
	}

	req := pb.ReadOrientationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapProtoAll(c.client.ReadOrientation(context.TODO(), &req))
}

func (c ReadAccelerationCapturer) Capture(params map[string]string) (*any.Any, error) {
	name, ok := params["name"]
	if !ok {
		return nil, errors.New("must pass name parameter to ReadAcceleration")
	}

	req := pb.ReadAccelerationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapProtoAll(c.client.ReadAcceleration(context.TODO(), &req))
}

func NewReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration, target *os.File) data.Collector {
	c := ReadAngularVelocityCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, params, target)
}

func NewReadOrientationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration, target *os.File) data.Collector {
	c := ReadOrientationCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, params, target)
}

func NewReadAccelerationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration, target *os.File) data.Collector {
	c := ReadAccelerationCapturer{client: pb.NewIMUServiceClient(conn)}

	return data.NewCollector(c, interval, params, target)
}
