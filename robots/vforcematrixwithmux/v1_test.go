package vforcematrixwithmux

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/inject"
)

func createExpectedMatrix(component config.Component) [][]int {
	numCols := len(component.Attributes.StringSlice("column_gpio_pins_left_to_right"))
	numRows := len(component.Attributes.IntSlice("io_pins_top_to_bottom"))
	expectedMatrix := make([][]int, numRows)
	for row := 0; row < numRows; row++ {
		expectedMatrix[row] = make([]int, numCols)
		for col := 0; col < numCols; col++ {
			expectedMatrix[row][col] = row + numRows*col
		}
	}
	return expectedMatrix
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
		component := config.Component{Attributes: config.AttributeMap{"analog_channel": "fake"}}
		_, err := New(context.Background(), fakeRobot, component, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect setMuxGpioPin to return error for bad ioPin", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeAR := &inject.AnalogReader{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return fakeAR, true
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		component := config.Component{Attributes: config.AttributeMap{"analog_channel": "fake"}}
		mux, _ := New(context.Background(), fakeRobot, component, logger)
		err := mux.setMuxGpioPins(context.Background(), -1)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("expect the matrix function to return a properly shaped object", func(t *testing.T) {
		t.Run("given a square matrix with size (4x4)", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			analogValue := 0
			fakeAR := &inject.AnalogReader{}
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			fakeAttrMap := config.AttributeMap{
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2", "3", "4"},
				"mux_gpio_pins_s2_to_s0":                []interface{}{"s2", "s1", "s0"},
				"io_pins_top_to_bottom":                 []interface{}{0, 2, 6, 7},
				"analog_reader":                         "fake",
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix := createExpectedMatrix(component)

			mux, _ := New(context.Background(), fakeRobot, component, logger)
			actualMatrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("given a rectangular matrix with size (2x6)", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			fakeAR := &inject.AnalogReader{}
			analogValue := 0
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			fakeAttrMap := config.AttributeMap{
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2", "3", "4", "5", "6"},
				"mux_gpio_pins_s2_to_s0":                []interface{}{"s2", "s1", "s0"},
				"io_pins_top_to_bottom":                 []interface{}{0, 2},
				"analog_reader":                         "fake",
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix := createExpectedMatrix(component)

			mux, _ := New(context.Background(), fakeRobot, component, logger)
			actualMatrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("given a rectangular matrix with size (5x3)", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			fakeAR := &inject.AnalogReader{}
			analogValue := 0
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			fakeAttrMap := config.AttributeMap{
				"column_gpio_pins_left_to_right":        []interface{}{"1", "2", "3"},
				"mux_gpio_pins_s2_to_s0":                []interface{}{"s2", "s1", "s0"},
				"io_pins_top_to_bottom":                 []interface{}{0, 2, 6, 7, 3},
				"analog_reader":                         "fake",
				"slip_detection_window":                 2,
				"slip_detection_signal_to_noise_cutoff": 150.,
			}
			component := config.Component{Attributes: fakeAttrMap}
			expectedMatrix := createExpectedMatrix(component)

			mux, _ := New(context.Background(), fakeRobot, component, logger)
			matrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, matrix, test.ShouldResemble, expectedMatrix)

			t.Run("expect slip detection to work and to return false at first", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

	})
}
