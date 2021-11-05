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

func createWorkingForceMatrix() *inject.ForceMatrix {
	forceMatrix := &inject.ForceMatrix{}
	numIterations := 0
	forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		rows, cols := 3, 4
		matrix := make([][]int, rows)
		if numIterations < NumMeasurements {
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

	t.Run("test calibrate function", func(t *testing.T) {
		rand.Seed(time.Now().UnixNano())

		t.Run("return error when pressure is the same for the open and closed position", func(t *testing.T) {
			fakeRobot := createWorkingRobotWithoutForceMatrix()
			forceMatrix := &inject.ForceMatrix{}
			forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
				return [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil
			}
			fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
				return forceMatrix, true
			}

			_, err := New(context.Background(), fakeRobot, config.Component{}, logger)
			test.That(t, err, test.ShouldNotBeNil)
		})

		t.Run("return error when open or closed directions are not successfully specified because"+
			"the pressureLimit doesn't divide them distinctly", func(t *testing.T) {
			fakeRobot := createWorkingRobotWithoutForceMatrix()
			forceMatrix := &inject.ForceMatrix{}
			pressureLimit := 500
			numIterations := 0
			forceMatrix.MatrixFunc = func(ctx context.Context) ([][]int, error) {
				rows, cols := 3, 4
				matrix := make([][]int, rows)
				if numIterations < NumMeasurements {
					for row := 0; row < rows; row++ {
						matrix[row] = make([]int, cols)
						for col := 0; col < cols; col++ {
							matrix[row][col] = rand.Intn(pressureLimit) + 501
						}
					}
				} else {
					for row := 0; row < rows; row++ {
						matrix[row] = make([]int, cols)
						for col := 0; col < cols; col++ {
							matrix[row][col] = rand.Intn(pressureLimit) + 501
						}
					}
				}
				numIterations++
				return matrix, nil
			}
			fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
				return forceMatrix, true
			}

			_, err := New(context.Background(), fakeRobot,
				config.Component{Attributes: config.AttributeMap{"pressureLimit": float64(pressureLimit)}}, logger)
			test.That(t, err, test.ShouldNotBeNil)
		})

		t.Run("expect no error when open and closed directions are correctly defined", func(t *testing.T) {
			fakeRobot := createWorkingRobotWithoutForceMatrix()
			forceMatrix := createWorkingForceMatrix()
			pressureLimit := 500
			fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
				return forceMatrix, true
			}

			fakeGripper, err := New(context.Background(), fakeRobot,
				config.Component{Attributes: config.AttributeMap{"pressureLimit": float64(pressureLimit)}}, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, fakeGripper.closedDirection != fakeGripper.openDirection, test.ShouldBeTrue)
		})
	})
}

func TestOpen(t *testing.T) {
	logger := golog.NewTestLogger(t)
	actualPosition := 5.
	failedPosition := actualPosition + 2*PositionTolerance
	successfulPosition := actualPosition + 1/2*PositionTolerance

	// Test Open
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
	forceMatrix := createWorkingForceMatrix()
	pressureLimit := 500.
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return forceMatrix, true
	}

	fakeGripper, err := New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"pressureLimit": pressureLimit}}, logger)
	test.That(t, err, test.ShouldBeNil)

	// Return error when the position of the fingers isn't within the allowed tolerance close to the open position.
	fakeGripper.openPos = failedPosition
	err = fakeGripper.Open(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	// Expect the position of the fingers to be close to the position of the openPosition
	fakeGripper.openPos = successfulPosition
	err = fakeGripper.Open(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Return error when the open position isn't reached before the timeout
	fakeRobot = &inject.Robot{}
	fakeBoard = createWorkingBoard()
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeMotor = createWorkingMotor()
	fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return actualPosition, nil
	}
	fakeMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return fakeMotor, true
	}
	// No error when open and closed directions are correctly defined
	forceMatrix = createWorkingForceMatrix()
	pressureLimit = 500.
	fakeRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return forceMatrix, true
	}

	_, err = New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"pressureLimit": pressureLimit}}, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGrab(t *testing.T) {
	// Test Grab
	// Expect the position of the fingers to be close to the position of the closedPosition
	// ? Test timeout?
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
