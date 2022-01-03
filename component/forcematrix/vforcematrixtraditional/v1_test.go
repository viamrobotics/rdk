package vforcematrixtraditional

import (
	"context"
	"strconv"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/forcematrix"
	"go.viam.com/rdk/testutils/inject"
)

func createExpectedMatrix(c *ForceMatrixConfig) ([][]int, error) {
	analogChannels := c.RowAnalogChannels
	numCols := len(c.ColumnGPIOPins)
	numRows := len(analogChannels)
	expectedMatrix := make([][]int, numRows)
	for row := 0; row < numRows; row++ {
		expectedMatrix[row] = make([]int, numCols)
		val, err := strconv.ParseInt(analogChannels[row], 10, 64)
		if err != nil {
			return expectedMatrix, err
		}
		for col := 0; col < numCols; col++ {
			expectedMatrix[row][col] = int(val)
		}
	}
	return expectedMatrix, nil
}

func TestNewForceMatrix(t *testing.T) {
	validConfig := &ForceMatrixConfig{
		BoardName:           "board",
		ColumnGPIOPins:      []string{"io10", "io11", "io12"},
		RowAnalogChannels:   []string{"a1"},
		SlipDetectionWindow: 10,
		NoiseThreshold:      5,
	}

	t.Run("valid", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeAnalogReader := &inject.AnalogReader{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return fakeAnalogReader, true
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fsm, err := newForceMatrix(fakeRobot, validConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fsm, test.ShouldNotBeNil)
		test.That(t, fsm.columnGpioPins, test.ShouldResemble, validConfig.ColumnGPIOPins)
		test.That(t, fsm.analogChannels, test.ShouldResemble, validConfig.RowAnalogChannels)
		test.That(t, len(fsm.previousMatrices), test.ShouldBeZeroValue)
		test.That(t, fsm.slipDetectionWindow, test.ShouldEqual, validConfig.SlipDetectionWindow)
		test.That(t, fsm.noiseThreshold, test.ShouldEqual, validConfig.NoiseThreshold)
	})

	t.Run("board not found", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := newForceMatrix(fakeRobot, validConfig)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("analog reader not found", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return nil, false
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		_, err := newForceMatrix(fakeRobot, validConfig)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		validConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			RowAnalogChannels:   []string{"a1"},
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := validConfig.Validate("path")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no board", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			RowAnalogChannels:   []string{"a1"},
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"board" is required`)
	})

	t.Run("no column gpio pins", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{},
			RowAnalogChannels:   []string{"a1"},
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
		test.That(t, err.Error(), test.ShouldContainSubstring, `column_gpio_pins_left_to_right has to be an array of length > 0`)
	})

	t.Run("no row analog channels", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			RowAnalogChannels:   []string{},
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
		test.That(t, err.Error(), test.ShouldContainSubstring, `row_analog_channels_top_to_bottom has to be an array of length > 0`)
	})

	t.Run("invalid slip detection window", func(t *testing.T) {
		t.Run("too small", func(t *testing.T) {
			invalidConfig := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"io10", "io11", "io12"},
				RowAnalogChannels:   []string{"a1"},
				SlipDetectionWindow: 0,
				NoiseThreshold:      5,
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
			test.That(t, err.Error(), test.ShouldContainSubstring, `slip_detection_window has to be: 0 < slip_detection_window <=`)
		})
		t.Run("too big", func(t *testing.T) {
			invalidConfig := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"io10", "io11", "io12"},
				RowAnalogChannels:   []string{"a1"},
				SlipDetectionWindow: forcematrix.MatrixStorageSize + 1,
				NoiseThreshold:      5,
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
			test.That(t, err.Error(), test.ShouldContainSubstring, `slip_detection_window has to be: 0 < slip_detection_window <=`)
		})
	})
}

func TestMatrixAndSlip(t *testing.T) {
	t.Run("correct shape", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			fakeAR := &inject.AnalogReader{}
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				val, err := strconv.ParseInt(name, 10, 64)
				if err != nil {
					return 0, err
				}
				return int(val), nil
			}
			return fakeAR, true
		}
		fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			return nil
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}

		t.Run("4x4", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4"},
				RowAnalogChannels:   []string{"1", "2", "3", "4"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := newForceMatrix(fakeRobot, config)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				isSlipping, err := fsm.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("3x7", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3"},
				RowAnalogChannels:   []string{"1", "2", "3", "4", "5", "6", "7"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := newForceMatrix(fakeRobot, config)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				isSlipping, err := fsm.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("5x2", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4", "5"},
				RowAnalogChannels:   []string{"1", "2"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := newForceMatrix(fakeRobot, config)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				isSlipping, err := fsm.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})
	})
}
