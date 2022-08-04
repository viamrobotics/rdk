package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/movementsensor"
)

// MovementSensor is an injected MovementSensor
type GPS struct {
	movementsensor.MovementSensor
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc          func(ctx context.Context) error
}

// Close calls the injected Close or the real version.
func (i *MovementSensor) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.MovementSensor)
	}
	return i.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (i *MovementSensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		return i.MovementSensor.Do(ctx, cmd)
	}
	return i.DoFunc(ctx, cmd)
}
