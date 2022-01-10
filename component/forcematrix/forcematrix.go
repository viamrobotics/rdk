// Package forcematrix defines the interface of a generic Force Matrix Sensor
// which provides a 2-dimensional array of integers that correlate to forces
// applied to the sensor.
package forcematrix

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "forcematrix".
const SubtypeName = resource.SubtypeName("forcematrix")

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
	sensor.Sensor
	Matrix(ctx context.Context) ([][]int, error)
	IsSlipping(ctx context.Context) (bool, error)
}

var (
	_ = ForceMatrix(&reconfigurableForceMatrix{})
	_ = resource.Reconfigurable(&reconfigurableForceMatrix{})
)

type reconfigurableForceMatrix struct {
	mu     sync.RWMutex
	actual ForceMatrix
}

func (r *reconfigurableForceMatrix) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableForceMatrix) Matrix(ctx context.Context) ([][]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Matrix(ctx)
}

func (r *reconfigurableForceMatrix) IsSlipping(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.IsSlipping(ctx)
}

func (r *reconfigurableForceMatrix) Readings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx)
}

func (r *reconfigurableForceMatrix) Desc() sensor.Description {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Desc()
}

func (r *reconfigurableForceMatrix) Reconfigure(ctx context.Context,
	newForceMatrix resource.Reconfigurable) error {
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
		return nil, errors.Errorf("expected resource to be ForceMatrix but got %T", r)
	}
	if reconfigurable, ok := fm.(*reconfigurableForceMatrix); ok {
		return reconfigurable, nil
	}
	return &reconfigurableForceMatrix{actual: fm}, nil
}
