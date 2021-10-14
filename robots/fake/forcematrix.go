package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/forcematrix"
)

func init() {
	registry.RegisterSensor(forcematrix.Type, ModelName, registry.Sensor{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
			return &ForceMatrix{Name: config.Name}, nil
		}})
}

// ForceMatrix is a fake ForceMatrix that always returns the same matrix of values
type ForceMatrix struct {
	Name string
}

// Matrix always returns the same matrix
func (fsm *ForceMatrix) Matrix(ctx context.Context) (matrix [][]int, error error) {
	result := make([][]int, 4)
	for i := 0; i < len(result); i++ {
		result[i] = []int{1, 1, 1, 1}
	}
	return result, nil
}

// Readings always returns the same values.
func (fsm *ForceMatrix) Readings(ctx context.Context) ([]interface{}, error) {
	matrix, err := fsm.Matrix(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{matrix}, nil
}

// Desc returns that this is a traditional compass.
func (fsm *ForceMatrix) Desc() sensor.Description {
	return sensor.Description{forcematrix.Type, ""}
}
