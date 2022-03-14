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

// readAngularVelocityCapturer is used when creating a Collector for an IMU's ReadAngularVelocity method.
type readAngularVelocityCapturer struct {
	client pb.IMUServiceClient
}

// readOrientationCapturer is used when creating a Collector for an IMU's ReadOrientation method.
type readOrientationCapturer struct {
	client pb.IMUServiceClient
}

// readAccelerationCapturer is used when creating a Collector for an IMU's ReadAcceleration method.
type readAccelerationCapturer struct {
	client pb.IMUServiceClient
}

// Capture returns an *any.Any containing the response of a single ReadAngularVelocity call on the backing client.
func (c readAngularVelocityCapturer) Capture(params map[string]string) (*any.Any, error) {
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

// Capture returns an *any.Any containing the response of a single ReadOrientation call on the backing client.
func (c readOrientationCapturer) Capture(params map[string]string) (*any.Any, error) {
	name, ok := params["name"]
	if !ok {
		return nil, errors.New("must pass name parameter to ReadOrientation")
	}

	req := pb.ReadOrientationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapProtoAll(c.client.ReadOrientation(context.TODO(), &req))
}

// Capture returns an *any.Any containing the response of a single ReadAcceleration call on the backing client.
func (c readAccelerationCapturer) Capture(params map[string]string) (*any.Any, error) {
	name, ok := params["name"]
	if !ok {
		return nil, errors.New("must pass name parameter to ReadAcceleration")
	}

	req := pb.ReadAccelerationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapProtoAll(c.client.ReadAcceleration(context.TODO(), &req))
}

func newReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := readAngularVelocityCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}

func newReadOrientationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := readOrientationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}

func newReadAccelerationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File) data.Collector {
	c := readAccelerationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target)
}
