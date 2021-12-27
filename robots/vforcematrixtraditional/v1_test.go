package vforcematrixtraditional

import (
	"context"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/inject"
)

func createExpectedMatrix(component config.Component) ([][]int, error) {
	analogChannels := component.Attributes.StringSlice("row_analog_channels_top_to_bottom")
	numCols := len(component.Attributes.StringSlice("column_gpio_pins_left_to_right"))
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

	t.Run("return error when not able to find board", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		component := config.Component{Attributes: config.AttributeMap{"type": "what"}}
		_, err := New(context.Background(), fakeRobot, component, logger)
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
		fakeRowReaders := []interface{}{"fakeRowReaderName"}
		component := config.Component{Attributes: config.AttributeMap{"row_analog_channels_top_to_bottom": fakeRowReaders}}
		_, err := New(context.Background(), fakeRobot, component, logger)
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
			fakeAttrMap := config.AttributeMap{
				"row_analog_channels_top_to_bottom":     []interface{}{"1", "2", "3", "4"},
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2", "3", "4"},
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix, err := createExpectedMatrix(component)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := New(context.Background(), fakeRobot, component, logger)
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
			fakeAttrMap := config.AttributeMap{
				"row_analog_channels_top_to_bottom":     []interface{}{"1", "2", "3"},
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2", "3", "4", "5", "6", "7"},
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix, err := createExpectedMatrix(component)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := New(context.Background(), fakeRobot, component, logger)
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
			fakeAttrMap := config.AttributeMap{
				"row_analog_channels_top_to_bottom":     []interface{}{"1", "2", "3", "4", "5"},
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2"},
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix, err := createExpectedMatrix(component)
			test.That(t, err, test.ShouldBeNil)

			fsm, _ := New(context.Background(), fakeRobot, component, logger)
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
