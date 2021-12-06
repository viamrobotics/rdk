package vforcematrixwithmux

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/testutils/inject"
)

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("return error when not able to find board", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := NewMux(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"type": "what"}}, logger)
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
		_, err := NewMux(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"analog_channel": "fake"}}, logger)
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
		mux, _ := NewMux(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"analog_channel": "fake"}}, logger)
		err := mux.setMuxGpioPins(context.Background(), -1)
		test.That(t, err, test.ShouldNotBeNil)

	})

	t.Run("expect the matrix function to return a properly shaped object", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeAR := &inject.AnalogReader{}
		fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
			return 1, nil
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
			"column_gpio_pins_left_to_right": []interface{}{"1", "2", "3", "4"},
			"mux_gpio_pins_s2_to_s0":         []interface{}{"s2", "s1", "s0"},
			"io_pins_top_to_bottom":          []interface{}{0, 2, 6, 7},
			"analog_reader":                  "fake",
			"slip_detection_window":          2,
		}
		component := config.Component{Attributes: fakeAttrMap}
		mux, _ := NewMux(context.Background(), fakeRobot, component, logger)
		expectedMatrix := make([][]int, 4)
		for i := 0; i < len(expectedMatrix); i++ {
			expectedMatrix[i] = []int{1, 1, 1, 1}
		}
		matrix, err := mux.Matrix(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, matrix, test.ShouldResemble, expectedMatrix)

		mux.Matrix(context.Background())
		mux.Matrix(context.Background())
		mux.Matrix(context.Background())
		isSlipping, err := mux.IsSlipping(context.Background())
		test.That(t, isSlipping, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

	})
}
