package vgripper

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func createWorkingMotor() *inject.LocalMotor {
	injectMotor := &inject.LocalMotor{}
	injectMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}
	injectMotor.GoTillStopFunc = func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
		return nil
	}
	injectMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}
	injectMotor.GoToFunc = func(ctx context.Context, rpm float64, position float64) error {
		return nil
	}
	injectMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}
	injectMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error {
		return nil
	}
	return injectMotor
}

// (kat) TODO:
// Test state and action mutexes behavior that have been added together with slip control.

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("return error when not able to find board", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
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

		_, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect the motor to support position measurements", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return &inject.Board{}, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := &inject.Motor{}
			fakeMotor.GetFeaturesFunc = func(ctx context.Context) (
				map[motor.Feature]bool, error,
			) {
				return map[motor.Feature]bool{}, nil
			}
			return fakeMotor, true
		}

		_, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect the motor to support GoTillStop", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return &inject.Board{}, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := &inject.Motor{}
			fakeMotor.GetFeaturesFunc = func(ctx context.Context) (
				map[motor.Feature]bool, error,
			) {
				return map[motor.Feature]bool{
					motor.PositionReporting: true,
				}, nil
			}
			return fakeMotor, true
		}
		motorName := "badMotor"
		cfg := config.Component{
			Attributes: config.AttributeMap{
				"motor": motorName,
			},
		}
		_, err := newGripper(context.Background(), fakeRobot, cfg, logger)
		test.That(t, err, test.ShouldBeError, motor.NewGoTillStopUnsupportedError(motorName))
	})

	t.Run("return error when not able to find current analog reader", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := &inject.Motor{}
			fakeMotor.GetFeaturesFunc = func(ctx context.Context) (
				map[motor.Feature]bool, error,
			) {
				return map[motor.Feature]bool{
					motor.PositionReporting: true,
				}, nil
			}
			return fakeMotor, true
		}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return nil, false
		}

		_, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
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
		fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			return nil, false
		}

		_, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("initializing gripper struct successful with proper parameters", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fakeRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			fakeMotor := &inject.LocalMotor{}
			fakeMotor.GetFeaturesFunc = func(ctx context.Context) (
				map[motor.Feature]bool, error,
			) {
				return map[motor.Feature]bool{
					motor.PositionReporting: true,
				}, nil
			}
			return fakeMotor, true
		}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return &inject.AnalogReader{}, true
		}
		// for forcematrix
		fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			return &inject.ForceMatrix{}, true
		}
		vg, err := newGripper(context.Background(), fakeRobot, config.Component{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vg, test.ShouldNotBeNil)
	})
}

func TestCalibrate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("return error when pressure is the same for the open and closed position", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeForceMatrix := &inject.ForceMatrix{}
		// return the same pressure no matter what
		hasPressureThreshold := 4.
		measuredPressure := 5
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}

		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			logger:                logger,
			numBadCurrentReadings: 0,
			hasPressureThreshold:  hasPressureThreshold,
		}
		err := injectedGripper.calibrate(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect no error when open and closed directions are correctly defined", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeForceMatrix := &inject.ForceMatrix{}

		openPressure := 0
		closedPressure := 10
		hasPressureThreshold := 5.

		called := -1
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			called++
			if called < numMeasurementsCalib {
				return [][]int{{openPressure}}, nil
			}
			return [][]int{{closedPressure}}, nil
		}

		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			logger:                logger,
			numBadCurrentReadings: 0,
			hasPressureThreshold:  hasPressureThreshold,
		}
		err := injectedGripper.calibrate(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestOpen(t *testing.T) {
	logger := golog.NewTestLogger(t)

	openPosition := 5.
	failedPosition := openPosition + 2*positionTolerance
	successfulPosition := openPosition + 0.5*positionTolerance

	t.Run("no error when position of fingers is within the allowed tolerance", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return successfulPosition, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			logger:                logger,
			numBadCurrentReadings: 0,
			openPos:               openPosition,
			state:                 gripperStateUnspecified,
		}
		err := injectedGripper.Open(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("return error when position of fingers is not within the allowed tolerance", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return failedPosition, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			logger:                logger,
			numBadCurrentReadings: 0,
			openPos:               openPosition,
			state:                 gripperStateUnspecified,
		}
		err := injectedGripper.Open(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("return error when the open position isn't reached before the timeout", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		// The motor will always be running, until the function hits the timeout
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			current:               fakeCurrent,
			logger:                logger,
			numBadCurrentReadings: 0,
			state:                 gripperStateUnspecified,
		}
		err := injectedGripper.Open(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		// Make sure the motor is off
		err = injectedGripper.motor.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestGrab(t *testing.T) {
	logger := golog.NewTestLogger(t)

	closedPosition := 5.
	failedPosition := closedPosition + 2*positionTolerance
	successfulPosition := closedPosition + 0.5*positionTolerance
	startHoldingPressure := 15.

	// Expect the position of the fingers to be close to the position of the closedPosition
	// or to have pressure on them
	// 1. not on + no pressure + not closed ==> error (why did motor stop in mid air?)
	t.Run("return error when motor stops mid-air while closing the gripper", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		// The motor stopped
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		// Gripper didn't reach the closed position
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return failedPosition, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}

		// There is no pressure detected
		measuredPressure := 0
		hasPressureThreshold := 500.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			current:               fakeCurrent,
			logger:                logger,
			numBadCurrentReadings: 0,
			hasPressureThreshold:  hasPressureThreshold,
			state:                 gripperStateUnspecified,
			startHoldingPressure:  startHoldingPressure,
		}
		grabbedSuccessfully, err := injectedGripper.Grab(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, grabbedSuccessfully, test.ShouldBeFalse)
	})

	// 2. not on --> didn't grab anything & closed all the way: false, nil
	t.Run("return false but no error when gripper closed completely without grabbing anything", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		// The motor stopped
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}
		// Gripper didn't reach the closed position
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return successfulPosition, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}

		// Doesn't matter if pressure is detected, cause it's not reliable right now
		// so let's assume it's not detected even though the gripper is closed.
		// (Since this depends on the actual physical system design).
		measuredPressure := 0
		hasPressureThreshold := 500.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			current:               fakeCurrent,
			logger:                logger,
			numBadCurrentReadings: 0,
			closedPos:             closedPosition,
			hasPressureThreshold:  hasPressureThreshold,
			state:                 gripperStateUnspecified,
			startHoldingPressure:  startHoldingPressure,
		}
		grabbedSuccessfully, err := injectedGripper.Grab(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, grabbedSuccessfully, test.ShouldBeFalse)
	})

	// Test successful grabbing if gripper detects pressure
	// 3. on + pressure --> true, error depends on motor.Go; let's test a successful grab
	t.Run("return (true, nil) when something is successfully grabbed", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		// The motor is still running
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		// Gripper didn't reach the closed position since it now holds an object
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return failedPosition, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}

		// There is pressure detected, since the gripper holds an object
		measuredPressure := 1000
		hasPressureThreshold := 500.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			current:               fakeCurrent,
			logger:                logger,
			numBadCurrentReadings: 0,
			closedPos:             closedPosition,
			hasPressureThreshold:  hasPressureThreshold,
			state:                 gripperStateUnspecified,
			startHoldingPressure:  startHoldingPressure,
		}
		grabbedSuccessfully, err := injectedGripper.Grab(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, grabbedSuccessfully, test.ShouldBeTrue)
		// Make sure that the motor is still on after it detected pressure & is holding the object
		motorIsOn, err := injectedGripper.motor.IsPowered(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, motorIsOn, test.ShouldBeTrue)
	})

	t.Run("return error when grabbing or closing wasn't successful before the timeout", func(t *testing.T) {
		fakeMotor := createWorkingMotor()
		// The motor will always be running, until the function hits the timeout
		fakeMotor.IsPoweredFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, nil
		}
		// There is no pressure detected
		measuredPressure := 0
		hasPressureThreshold := 500.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{measuredPressure}}, nil
		}
		injectedGripper := &gripperV2{
			motor:                 fakeMotor,
			forceMatrix:           fakeForceMatrix,
			current:               fakeCurrent,
			logger:                logger,
			numBadCurrentReadings: 0,
			closedPos:             closedPosition,
			hasPressureThreshold:  hasPressureThreshold,
			state:                 gripperStateUnspecified,
			startHoldingPressure:  startHoldingPressure,
		}
		grabbedSuccessfully, err := injectedGripper.Grab(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, grabbedSuccessfully, test.ShouldBeFalse)
		// Make sure the motor is off
		err = injectedGripper.motor.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestProcessCurrentReading(t *testing.T) {
	// MaxCurrent = 6.0
	// CurrentBadReadingCounts = 50
	t.Run("when current is too high but not too often yet", func(t *testing.T) {
		current := maxCurrent + 1.0
		injectedGripper := &gripperV2{
			numBadCurrentReadings: currentBadReadingCounts - 2,
		}
		err := injectedGripper.checkCurrentInAcceptableRange(current, "testing")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("return error when the current is too high for too long", func(t *testing.T) {
		current := maxCurrent + 1.0
		injectedGripper := &gripperV2{
			numBadCurrentReadings: currentBadReadingCounts - 1,
		}
		err := injectedGripper.checkCurrentInAcceptableRange(current, "testing")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("reset numBadCurrentReadings when current is in the healthy range", func(t *testing.T) {
		current := 0.0
		injectedGripper := &gripperV2{
			numBadCurrentReadings: currentBadReadingCounts - 5,
		}
		err := injectedGripper.checkCurrentInAcceptableRange(current, "testing")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, injectedGripper.numBadCurrentReadings, test.ShouldEqual, 0)
	})
}

func TestClose(t *testing.T) {
	t.Run("make sure calling Close shuts down the motor", func(t *testing.T) {
		fakeMotor := &inject.LocalMotor{}
		counter := 0
		fakeMotor.StopFunc = func(ctx context.Context) error {
			counter++
			return nil
		}
		injectedGripper := &gripperV2{
			motor: fakeMotor,
		}
		err := injectedGripper.Close(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, counter, test.ShouldEqual, 1)
	})
}

func TestStop(t *testing.T) {
	t.Run("make sure calling Stops shuts down the motor", func(t *testing.T) {
		fakeMotor := &inject.LocalMotor{}
		counter := 0
		fakeMotor.StopFunc = func(ctx context.Context) error {
			counter++
			return nil
		}
		injectedGripper := &gripperV2{
			motor: fakeMotor,
		}
		err := injectedGripper.Stop(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, counter, test.ShouldEqual, 1)
	})
}

func TestReadCurrent(t *testing.T) {
	measuredCurrent := 768
	fakeCurrent := &inject.AnalogReader{}
	fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
		return measuredCurrent, nil
	}
	injectedGripper := &gripperV2{
		current: fakeCurrent,
	}
	current, err := injectedGripper.readCurrent(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, current, test.ShouldAlmostEqual, ((float64(measuredCurrent)*(3.3/1023))-1.12)/0.2)
}

func TestReadRobustAveragePressure(t *testing.T) {
	t.Run("successfully read the average pressure", func(t *testing.T) {
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{1, 2}, {3, 4}}, nil
		}
		injectedGripper := &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		// All 5 measurements the same
		averagePressure, err := injectedGripper.readRobustAveragePressure(context.Background(), 5)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, averagePressure, test.ShouldAlmostEqual, 2.5)

		// Let's add more variation to the measurements
		counter := 0
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			counter++
			switch counter {
			case 1:
				// sum: 10; average: 2.5
				return [][]int{{1, 2}, {3, 4}}, nil
			case 2:
				// sum: 28; average: 7
				return [][]int{{5, 6}, {9, 8}}, nil
			case 3:
				// sum: 4; average: 1
				return [][]int{{1, 1}, {1, 1}}, nil
			case 4:
				// sum: 196; average: 49
				return [][]int{{101, 3}, {15, 77}}, nil
			default:
				return [][]int{{0, 0}, {0, 0}}, errors.New("this case shouldn't happen")
			}
		}
		injectedGripper = &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		averagePressure, err = injectedGripper.readRobustAveragePressure(context.Background(), 4)
		test.That(t, err, test.ShouldBeNil)
		// (2.5 + 7 + 1 + 49)/4 = 14.875
		test.That(t, averagePressure, test.ShouldAlmostEqual, 14.875)
	})

	t.Run("return error when reading the matrix went wrong", func(t *testing.T) {
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{}}, errors.New("matrix reading failed")
		}
		injectedGripper := &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		averagePressure, err := injectedGripper.readRobustAveragePressure(context.Background(), 4)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, averagePressure, test.ShouldAlmostEqual, 0)
	})
}

func TestReadAveragePressure(t *testing.T) {
	t.Run("successfully read the average pressure", func(t *testing.T) {
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{1, 2}, {3, 4}}, nil
		}
		injectedGripper := &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		averagePressure, err := injectedGripper.readAveragePressure(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, averagePressure, test.ShouldAlmostEqual, 2.5)
	})

	t.Run("return error when reading the matrix went wrong", func(t *testing.T) {
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{}}, errors.New("matrix reading failed")
		}
		injectedGripper := &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		averagePressure, err := injectedGripper.readAveragePressure(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, averagePressure, test.ShouldAlmostEqual, 0)
	})
}

func TestHasPressure(t *testing.T) {
	t.Run("detect pressure", func(t *testing.T) {
		hasPressureThreshold := 1.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{1, 2}, {3, 4}}, nil
		}
		injectedGripper := &gripperV2{
			forceMatrix:          fakeForceMatrix,
			hasPressureThreshold: hasPressureThreshold,
		}
		hasPressure, pressure, err := injectedGripper.hasPressure(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hasPressure, test.ShouldBeTrue)
		test.That(t, pressure, test.ShouldAlmostEqual, 2.5)
	})

	t.Run("don't detect pressure", func(t *testing.T) {
		hasPressureThreshold := 10.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{4, 10}, {6, 4}}, nil
		}
		injectedGripper := &gripperV2{
			forceMatrix:          fakeForceMatrix,
			hasPressureThreshold: hasPressureThreshold,
		}
		hasPressure, pressure, err := injectedGripper.hasPressure(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hasPressure, test.ShouldBeFalse)
		test.That(t, pressure, test.ShouldAlmostEqual, 6)
	})

	t.Run("return error when reading the matrix went wrong", func(t *testing.T) {
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{}}, errors.New("matrix reading failed")
		}
		injectedGripper := &gripperV2{
			forceMatrix: fakeForceMatrix,
		}
		hasPressure, pressure, err := injectedGripper.hasPressure(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, hasPressure, test.ShouldBeFalse)
		test.That(t, pressure, test.ShouldAlmostEqual, 0)
	})
}

func TestAnalogs(t *testing.T) {
	t.Run("no error when everything reads successfully", func(t *testing.T) {
		rawCurrent := 256
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return rawCurrent, nil
		}
		hasPressureThreshold := 4.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{1, 5}, {6, 8}}, nil
		}
		injectedGripper := &gripperV2{
			current:              fakeCurrent,
			forceMatrix:          fakeForceMatrix,
			hasPressureThreshold: hasPressureThreshold,
		}
		hasPressure, pressure, current, err := injectedGripper.analogs(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hasPressure, test.ShouldBeTrue)
		test.That(t, pressure, test.ShouldAlmostEqual, 5)
		test.That(t, current, test.ShouldAlmostEqual, (((float64(rawCurrent) * (3.3 / 1023)) - 1.12) / 0.2))
	})

	t.Run("return error when reading the pressure went wrong", func(t *testing.T) {
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 10, nil
		}
		hasPressureThreshold := 4.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{}}, errors.New("matrix reading went wrong")
		}
		injectedGripper := &gripperV2{
			current:              fakeCurrent,
			forceMatrix:          fakeForceMatrix,
			hasPressureThreshold: hasPressureThreshold,
		}
		hasPressure, pressure, current, err := injectedGripper.analogs(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, hasPressure, test.ShouldBeFalse)
		test.That(t, pressure, test.ShouldAlmostEqual, 0)
		test.That(t, current, test.ShouldAlmostEqual, 0)
	})

	t.Run("return error when reading the current went wrong", func(t *testing.T) {
		fakeCurrent := &inject.AnalogReader{}
		fakeCurrent.ReadFunc = func(ctx context.Context) (int, error) {
			return 0, errors.New("current reading went wrong")
		}
		hasPressureThreshold := 4.
		fakeForceMatrix := &inject.ForceMatrix{}
		fakeForceMatrix.ReadMatrixFunc = func(ctx context.Context) ([][]int, error) {
			return [][]int{{1, 5}, {6, 8}}, nil
		}
		injectedGripper := &gripperV2{
			current:              fakeCurrent,
			forceMatrix:          fakeForceMatrix,
			hasPressureThreshold: hasPressureThreshold,
		}
		hasPressure, pressure, current, err := injectedGripper.analogs(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, hasPressure, test.ShouldBeFalse)
		test.That(t, pressure, test.ShouldAlmostEqual, 0)
		test.That(t, current, test.ShouldAlmostEqual, 0)
	})
}
