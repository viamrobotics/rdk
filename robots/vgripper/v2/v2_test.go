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

func createWorkingForceMatrix(pressureLimit int) *inject.ForceMatrix {
	rand.Seed(time.Now().UnixNano())
	forceMatrix := &inject.ForceMatrix{}
	numIterations := 0
	forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		rows, cols := 3, 4
		matrix := make([][]int, rows)
		if numIterations < NumMeasurementsCalib {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(1024-pressureLimit) + pressureLimit
				}
			}
		} else {
			for row := 0; row < rows; row++ {
				matrix[row] = make([]int, cols)
				for col := 0; col < cols; col++ {
					matrix[row][col] = rand.Intn(pressureLimit)
				}
			}
		}
		numIterations++
		return matrix, nil
	}
	return forceMatrix
}

func createWorkingMotor() *inject.Motor {
	fakeMotor := &inject.Motor{}
	fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	fakeMotor.GoTillStopFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
		return nil
	}
	fakeMotor.OffFunc = func(ctx context.Context) error {
		return nil
	}
	fakeMotor.GoToFunc = func(ctx context.Context, rpm float64, position float64) error {
		return nil
	}
	return fakeMotor
}

func createWorkingBoard() *inject.Board {
	fakeBoard := &inject.Board{}
	fakeAnalogReader := &inject.AnalogReader{}
	fakeAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
		return 0, nil
	}

	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return fakeAnalogReader, true
	}
	return fakeBoard
}

func createWorkingRobotWithoutForceMatrix() *inject.Robot {
	fakeRobot := &inject.Robot{}

	fakeBoard := createWorkingBoard()
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}

	fakeMotor := createWorkingMotor()
	fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 0, nil
	}
	fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}

	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return fakeMotor, true
	}
	return fakeRobot

}

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("return error when not able to find board", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)

	})

	t.Run("return error when not able to find motor", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return &inject.Board{}, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			return nil, false
		}

		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect the motor to support position measurements", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
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

		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)

	})

	t.Run("return error when not able to find current analog reader", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
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

		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("return error when not able to find forcematrix", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := createWorkingMotor()
			return fakeMotor, true
		}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return &inject.AnalogReader{}, true
		}
		fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return nil, false
		}

		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("return error when returned sensor is not a forcematrix", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := createWorkingMotor()
			return fakeMotor, true
		}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return &inject.AnalogReader{}, true
		}
		fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return &inject.Sensor{}, true
		}

		_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestCalibrate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("return error when pressure is the same for the open and closed position", func(t *testing.T) {
		fakeMotor := &inject.Motor{}
		fakeMotor.GoTillStopFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
			return nil
		}
		fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeForceMatrix := &inject.ForceMatrix{}
		// return the same pressure no matter what
		pressureLimit := 4.
		measuredPressure := 5
		fakeForceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}

		injectedGripper := &GripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			logger:                logger,
			numBadCurrentReadings: 0,
			pressureLimit:         pressureLimit,
		}
		err := injectedGripper.calibrate(context.Background(), logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect no error when open and closed directions are correctly defined", func(t *testing.T) {
		fakeMotor := &inject.Motor{}
		fakeMotor.GoTillStopFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
			return nil
		}
		fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeForceMatrix := &inject.ForceMatrix{}

		openPressure := 0
		closedPressure := 10
		pressureLimit := 5.

		called := 0
		fakeForceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
			if called < NumMeasurementsCalib {
				called++
				return [][]int{{openPressure}}, nil
			} else {
				called++
				return [][]int{{closedPressure}}, nil
			}
		}

		injectedGripper := &GripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			logger:                logger,
			numBadCurrentReadings: 0,
			pressureLimit:         pressureLimit,
		}
		err := injectedGripper.calibrate(context.Background(), logger)
		test.That(t, err, test.ShouldBeNil)
	})

}

func TestOpen(t *testing.T) {
	logger := golog.NewTestLogger(t)

	actualPosition := 5.
	failedPosition := actualPosition + 2*PositionTolerance
	successfulPosition := actualPosition + 1/2*PositionTolerance

	t.Run("test position accuracy ensurance", func(t *testing.T) {
		// Setup
		fakeRobot := &inject.Robot{}
		fakeBoard := createWorkingBoard()
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}

		fakeMotor := createWorkingMotor()
		fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			return actualPosition, nil
		}
		fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			return fakeMotor, true
		}
		pressureLimit := 500
		fakeForceMatrix := createWorkingForceMatrix(pressureLimit)
		fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return fakeForceMatrix, true
		}

		fakeGripper, err := New(context.Background(), fakeRobot,
			config.Component{Attributes: config.AttributeMap{"pressureLimit": float64(pressureLimit)}}, logger)

		// Tests
		test.That(t, err, test.ShouldBeNil)
		t.Run("return error when the position of the fingers isn't within the allowed tolerance"+
			" close to the open position", func(t *testing.T) {
			fakeGripper.openPos = failedPosition
			err = fakeGripper.Open(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
		})

		t.Run("expect the position of the fingers to be within the tolerance close to the openPosition",
			func(t *testing.T) {
				fakeGripper.openPos = successfulPosition
				err = fakeGripper.Open(context.Background())
				test.That(t, err, test.ShouldBeNil)
			})

	})

	t.Run("return error when the open position isn't reached before the timeout", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			return actualPosition, nil
		}
		fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{0, 0, 0}, {0, 0, 0}, {0, 0, 0}}, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}

		injectedGripper := GripperV2{
			motor:                 fakeMotor,
			current:               fakeCurrent,
			forceMatrix:           fakeForceMatrix,
			openPos:               actualPosition,
			openDirection:         pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			logger:                logger,
			numBadCurrentReadings: 0,
		}

		err := injectedGripper.Open(context.Background())
		test.That(t, err, test.ShouldNotBeNil)

	})
}

func TestGrab(t *testing.T) {
	logger := golog.NewTestLogger(t)

	actualPosition := 5.
	failedPosition := actualPosition + 2*PositionTolerance
	// successfulPosition := actualPosition + 1/2*PositionTolerance

	// Expect the position of the fingers to be close to the position of the closedPosition
	// or to have pressure on them
	// 1. not on + no pressure + not closed ==> error (why did motor stop in mid air?)
	t.Run("return error when gripper motor stops mid-air while gripping", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := createWorkingBoard()
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}

		fakeMotor := createWorkingMotor()
		fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			return actualPosition, nil
		}
		// The gripper is not on
		fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			return fakeMotor, true
		}
		pressureLimit := 500
		fakeForceMatrix := createWorkingForceMatrix(pressureLimit)
		fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return fakeForceMatrix, true
		}

		fakeGripper, _ := New(context.Background(), fakeRobot,
			config.Component{Attributes: config.AttributeMap{"pressureLimit": float64(pressureLimit)}}, logger)

		// The gripper is not closed
		fakeGripper.closedPos = failedPosition

	})
	// 2. not on --> didn't grab anything & closed all the way: false, nil

	// Test successful grabbing if gripper detects pressure
	// 3. on + pressure --> true, error depends on motor.Go; i can set it to nil or true; test for both!

	// Make sure that the motor is still on after it detected pressure & is holding the object
	// 4. after testing the above, proof this

	// Test timeout

}

func TestProcessCurrentReading(t *testing.T) {
	// Test processCurrentReading
	// ? Can I change current & MaxCurrent and thus test for the correct response?
}

func TestClose(t *testing.T) {

}

func TestStop(t *testing.T) {

}

// Test readCurrent

// Test readRobustAveragePressure

// Test readAveragePressure

// Test hasPressure

// Test analogs
