package imu

import (
	"context"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
)

// ReadAngularVelocityCapturer is used when creating a Collector for an IMU's ReadAngularVelocity method.
type ReadAngularVelocityCapturer struct {
	client pb.IMUServiceClient
}

// ReadOrientationCapturer is used when creating a Collector for an IMU's ReadOrientation method.
type ReadOrientationCapturer struct {
	client pb.IMUServiceClient
}

// ReadAccelerationCapturer is used when creating a Collector for an IMU's ReadAcceleration method.
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

func NewReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := ReadAngularVelocityCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}

func NewReadOrientationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := ReadOrientationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}

func NewReadAccelerationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := ReadAccelerationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}
