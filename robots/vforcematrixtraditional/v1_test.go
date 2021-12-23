package vforcematrixtraditional

import (
	"context"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
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

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	validConfig := &ForceMatrixConfig{
		BoardName:           "board",
		ColumnGPIOPins:      []string{"io10", "io11", "io12"},
		RowAnalogChannels:   []string{"a1"},
		SlipDetectionWindow: 10,
		NoiseThreshold:      5,
	}

	t.Run("return error when not able to find board", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := new(context.Background(), fakeRobot, validConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("return error when unable to find analog reader", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return nil, false
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		_, err := new(context.Background(), fakeRobot, validConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect the matrix function to return properly shaped object", func(t *testing.T) {
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

		t.Run("given a square matrix with size (4x4)", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4"},
				RowAnalogChannels:   []string{"1", "2", "3", "4"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := new(context.Background(), fakeRobot, config, logger)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				isSlipping, err := fsm.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("given a rectangular matrix with size (3x7)", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3"},
				RowAnalogChannels:   []string{"1", "2", "3", "4", "5", "6", "7"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := new(context.Background(), fakeRobot, config, logger)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				fsm.Matrix(context.Background())
				isSlipping, err := fsm.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("given a rectangular matrix with size (5x2)", func(t *testing.T) {
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4", "5"},
				RowAnalogChannels:   []string{"1", "2"},
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix, err := createExpectedMatrix(config)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := new(context.Background(), fakeRobot, config, logger)
			actualMatrix, err := fsm.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
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
