package arduino

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func TestArduinoMotorInit(t *testing.T) {
	logger := golog.NewTestLogger(t)
	motorReg := registry.ComponentLookup(motor.Subtype, "arduino")
	test.That(t, motorReg, test.ShouldNotBeNil)

	t.Run("initialization failure on config without board name", func(t *testing.T) {
		emptyConfig := config.Component{
			Model:               "arduino",
			SubType:             motor.Subtype.String(),
			ConvertedAttributes: &motor.Config{},
		}
		deps := make(registry.Dependencies)
		_motor, err := motorReg.Constructor(
			context.Background(), deps, emptyConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, _motor, test.ShouldBeNil)
	})

	t.Run("initialization failure when unable to retrieve board", func(t *testing.T) {
		badBoardConfig := config.Component{
			Model:   "arduino",
			SubType: motor.Subtype.String(),
			ConvertedAttributes: &motor.Config{
				BoardName: "oops no board",
			},
		}
		deps := make(registry.Dependencies)
		_motor, err := motorReg.Constructor(
			context.Background(), deps, badBoardConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, _motor, test.ShouldBeNil)
	})

	t.Run("initialization failure when board is not an arduino", func(t *testing.T) {
		badBoardConfig := config.Component{
			Model:   "arduino",
			SubType: motor.Subtype.String(),
			ConvertedAttributes: &motor.Config{
				BoardName: "non-arduino",
			},
		}
		deps := make(registry.Dependencies)
		_motor, err := motorReg.Constructor(
			context.Background(), deps, badBoardConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, _motor, test.ShouldBeNil)
	})
}
