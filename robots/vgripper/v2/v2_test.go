package vgripper

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/sensor"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/test"
)

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// Return error when not able to find board.
	fakeRobot := &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return nil, false
	}
	_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Return error when not able to find motor.
	fakeRobot = &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &inject.Board{}, true
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return nil, false
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Expect the motor to support position measurements.
	fakeRobot = &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &inject.Board{}, true
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		fakeMotor := &inject.Motor{}
		fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		return fakeMotor, true
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Return error when not able to find current analog reader.
	fakeRobot = &inject.Robot{}
	fakeBoard := &inject.Board{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		fakeMotor := &inject.Motor{}
		fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		return fakeMotor, true
	}
	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return nil, false
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Return error when not able to find forcematrix.
	fakeRobot = &inject.Robot{}
	fakeBoard = &inject.Board{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		fakeMotor := &inject.Motor{}
		fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		return fakeMotor, true
	}
	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return &inject.AnalogReader{}, true
	}
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return nil, false
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Return error when returned sensor is not a forcematrix.
	fakeRobot = &inject.Robot{}
	fakeBoard = &inject.Board{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		fakeMotor := &inject.Motor{}
		fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		return fakeMotor, true
	}
	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return &inject.AnalogReader{}, true
	}
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return &inject.Sensor{}, true
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Test after calling the calibrate function:
	// Expect vg.closedDirection and vg.openDirection to be specified successfully
	fakeRobot = &inject.Robot{}

	fakeBoard = &inject.Board{}
	fakeAnalogReader := &inject.AnalogReader{}
	fakeAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
		return 0, nil
	}

	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return fakeAnalogReader, true
	}

	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}

	fakeMotor := &inject.Motor{}
	fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	fakeMotor.GoTillStopFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
		return nil
	}
	fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 0, nil
	}
	fakeMotor.OffFunc = func(ctx context.Context) error {
		return nil
	}
	fakeMotor.GoToFunc = func(ctx context.Context, rpm float64, position float64) error {
		return nil
	}
	fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}

	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return fakeMotor, true
	}

	// Error when pressure is same for open and closed position
	forceMatrix := &inject.ForceMatrix{}
	forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		return [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil
	}
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return forceMatrix, true
	}

	_, err = New(context.Background(), fakeRobot, config.Component{}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Test if configuration is successful:
	rand.Seed(time.Now().UnixNano())

	// Error when open or closed directions are not successfully specified because
	// the pressureLimit doesn't divide them distinctly.
	forceMatrix = &inject.ForceMatrix{}
	pressureLimit := 500.
	numIterations := 0
	forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		rows, cols := 3, 4
		matrix := make([][]int, rows)
		if numIterations < numMeasurements {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(500) + 501
				}
			}
		} else {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(500) + 501
				}
			}
		}
		numIterations++
		return matrix, nil
	}
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return forceMatrix, true
	}

	_, err = New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"pressureLimit": pressureLimit}}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// No error when open and closed directions are correctly defined
	forceMatrix = &inject.ForceMatrix{}
	pressureLimit = 500.
	numIterations = 0
	forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		rows, cols := 3, 4
		matrix := make([][]int, rows)
		if numIterations < numMeasurements {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(500) + 501
				}
			}
		} else {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(300)
				}
			}
		}
		numIterations++
		return matrix, nil
	}
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return forceMatrix, true
	}

	fakeGripper, err := New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"pressureLimit": pressureLimit}}, logger)
	test.That(t, err, test.ShouldBeNil)
	// Expect vg.openDirection != vg.closedDirection
	test.That(t, fakeGripper.closedDirection != fakeGripper.openDirection, test.ShouldBeTrue)

	// Test Open
	// Expect the position of the fingers to be close to the position of the openPosition
	// TODO: Expect parameter that defines the allowed position error
	// ? Test timeout?

	// Test Grab
	// Expect the position of the fingers to be close to the position of the closedPosition
	// TODO: Expect parameter that defines the allowed position error (same as above)
	// ? Test timeout?

	// Test processCurrentReading
	// ? Can I change current & MaxCurrent and thus test for the correct response?

	// Test Close

	// Test stopAfterError

	// Test Stop

	// Test readCurrent

	// Test readRobustAveragePressure

	// Test readAveragePressure

	// Test hasPressure

	// Test analogs

}
