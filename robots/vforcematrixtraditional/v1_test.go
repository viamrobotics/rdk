package vforcematrixtraditional

import (
	"context"
	"strconv"
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
		_, err := New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"type": "what"}}, logger)
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
		_, err := New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"row_analog_channels_top_to_bottom": fakeRowReaders}}, logger)
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
		fakeRowReaders := []interface{}{"1", "2", "3", "4"}
		fakeColumnGpioPins := []interface{}{"1", "2", "3", "4"}
		fakeAttrMap := config.AttributeMap{
			"row_analog_channels_top_to_bottom": fakeRowReaders,
			"column_gpio_pins_left_to_right":    fakeColumnGpioPins,
		}
		component := config.Component{Attributes: fakeAttrMap}
		fsm, _ := New(context.Background(), fakeRobot, component, logger)
		matrix, err := fsm.Matrix(context.Background())
		test.That(t, err, test.ShouldBeNil)
		expectedMatrix := make([][]int, 4)
		for i := 0; i < len(expectedMatrix); i++ {
			val := i + 1
			expectedMatrix[i] = []int{val, val, val, val}
		}
		test.That(t, matrix, test.ShouldResemble, expectedMatrix)

	})
}
