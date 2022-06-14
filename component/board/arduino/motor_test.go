package arduino

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
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
		_robot := &inject.Robot{}
		_robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, utils.NewResourceNotFoundError(name)
		}
		_motor, err := motorReg.Constructor(
			context.Background(), _robot, badBoardConfig, logger)
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
		_robot := &inject.Robot{}
		_robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return &inject.Board{}, nil
		}
		_motor, err := motorReg.Constructor(
			context.Background(), _robot, badBoardConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, _motor, test.ShouldBeNil)
	})
}
