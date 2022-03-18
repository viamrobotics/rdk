package imu

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/protobuf/ptypes/any"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
)

const (
	paramName = "paramName"
)

type method int64

const (
	readAngularVelocity method = iota
	readOrientation
	readAcceleration
)

func (m method) String() string {
	switch m {
	case readAngularVelocity:
		return "ReadAngularVelocity"
	case readOrientation:
		return "ReadOrientation"
	case readAcceleration:
		return "ReadAcceleration"
	}
	return "Unknown"
}

type readAngularVelocityCapturer struct {
	client pb.IMUServiceClient
}

type readOrientationCapturer struct {
	client pb.IMUServiceClient
}

type readAccelerationCapturer struct {
	client pb.IMUServiceClient
}

// Capture returns an *any.Any containing the response of a single ReadAngularVelocity call on the backing client.
func (c readAngularVelocityCapturer) Capture(ctx context.Context, params map[string]string) (*any.Any, error) {
	name, ok := params[paramName]
	if !ok {
		return nil, data.MissingParameterErr(paramName, readAngularVelocity.String())
	}

	req := pb.ReadAngularVelocityRequest{
		Name: name,
	}
	// TODO: what should context be here?
	return data.WrapInAll(c.client.ReadAngularVelocity(ctx, &req))
}

// Capture returns an *any.Any containing the response of a single ReadOrientation call on the backing client.
func (c readOrientationCapturer) Capture(ctx context.Context, params map[string]string) (*any.Any, error) {
	name, ok := params[paramName]
	if !ok {
		return nil, data.MissingParameterErr(paramName, readOrientation.String())
	}

	req := pb.ReadOrientationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapInAll(c.client.ReadOrientation(ctx, &req))
}

// Capture returns an *any.Any containing the response of a single ReadAcceleration call on the backing client.
func (c readAccelerationCapturer) Capture(ctx context.Context, params map[string]string) (*any.Any, error) {
	name, ok := params[paramName]
	if !ok {
		return nil, data.MissingParameterErr(paramName, readAcceleration.String())
	}

	req := pb.ReadAccelerationRequest{Name: name}
	// TODO: what should context be here?
	return data.WrapInAll(c.client.ReadAcceleration(ctx, &req))
}

func newReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File, logger golog.Logger) data.Collector {
	c := readAngularVelocityCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target, logger)
}

func newReadOrientationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File, logger golog.Logger) data.Collector {
	c := readOrientationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target, logger)
}

func newReadAccelerationCollectorFromConn(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File, logger golog.Logger) data.Collector {
	c := readAccelerationCapturer{client: pb.NewIMUServiceClient(conn)}
	return data.NewCollector(c, interval, params, target, logger)
}
