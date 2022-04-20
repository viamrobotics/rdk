// Package forcematrix defines the interface of a generic Force Matrix Sensor
// which provides a 2-dimensional array of integers that correlate to forces
// applied to the sensor.
package forcematrix

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/forcematrix/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.ForceMatrixService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterForceMatrixServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string "force_matrix".
const SubtypeName = resource.SubtypeName("force_matrix")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named ForceMatrix's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// MatrixStorageSize determines how many matrices to store in history queue.
const MatrixStorageSize = 200

// A ForceMatrix represents a force sensor that outputs a 2-dimensional array
// with integers that correlate to the forces applied to the sensor.
type ForceMatrix interface {
	ReadMatrix(ctx context.Context) ([][]int, error)
	DetectSlip(ctx context.Context) (bool, error)
}

var (
	_ = ForceMatrix(&reconfigurableForceMatrix{})
	_ = sensor.Sensor(&reconfigurableForceMatrix{})
	_ = resource.Reconfigurable(&reconfigurableForceMatrix{})
)

// FromRobot is a helper for getting the named force matrix sensor from the given Robot.
func FromRobot(r robot.Robot, name string) (ForceMatrix, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(ForceMatrix)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("ForceMatrix", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all force matrix sensor names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// GetReadings is a helper for getting all readings from a force matrix sensor.
func GetReadings(ctx context.Context, f ForceMatrix) ([]interface{}, error) {
	matrix, err := f.ReadMatrix(ctx)
	if err != nil {
		return nil, err
	}
	// assume perfectly rectangular
	numRows := len(matrix)
	numCols := len(matrix[0])

	readings := make([]interface{}, 0, numRows*numCols+2)
	// number of rows, number of cols, then all the readings
	readings = append(readings, numRows, numCols)
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			readings = append(readings, matrix[row][col])
		}
	}
	return readings, nil
}

type reconfigurableForceMatrix struct {
	mu     sync.RWMutex
	actual ForceMatrix
}

func (r *reconfigurableForceMatrix) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableForceMatrix) ReadMatrix(ctx context.Context) ([][]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadMatrix(ctx)
}

func (r *reconfigurableForceMatrix) DetectSlip(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.DetectSlip(ctx)
}

// GetReadings will use the default ForceMatrix GetReadings if not provided.
func (r *reconfigurableForceMatrix) GetReadings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sensor, ok := r.actual.(sensor.Sensor); ok {
		return sensor.GetReadings(ctx)
	}
	return GetReadings(ctx, r.actual)
}

func (r *reconfigurableForceMatrix) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableForceMatrix) Reconfigure(ctx context.Context,
	newForceMatrix resource.Reconfigurable,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newForceMatrix.(*reconfigurableForceMatrix)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newForceMatrix)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular ForceMatrix implementation to a reconfigurableForceMatrix.
// If the ForceMatrix is already a reconfigurableForceMatrix, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	fm, ok := r.(ForceMatrix)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("ForceMatrix", r)
	}
	if reconfigurable, ok := fm.(*reconfigurableForceMatrix); ok {
		return reconfigurable, nil
	}
	return &reconfigurableForceMatrix{actual: fm}, nil
}
