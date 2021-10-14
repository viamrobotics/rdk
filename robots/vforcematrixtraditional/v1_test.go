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

	// returns error when not able to find board
	fakeRobot := &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return nil, false
	}
	_, err := New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"type": "what"}}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// returns error when unable to find analog reader
	fakeBoard := &inject.Board{}
	fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return nil, false
	}
	fakeRobot = &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeReaders := []interface{}{"fakeReaderName"}
	_, err = New(context.Background(), fakeRobot, config.Component{Attributes: config.AttributeMap{"row_analog_channels_top_to_bottom": fakeReaders}}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// matrix function returns properly shaped object
	fakeBoard = &inject.Board{}
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
	fakeRobot = &inject.Robot{}
	fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return fakeBoard, true
	}
	fakeReaders = []interface{}{"1", "2", "3", "4"}
	fakePins := []interface{}{"1", "2", "3", "4"}
	fakeAttrMap := config.AttributeMap{
		"row_analog_channels_top_to_bottom": fakeReaders,
		"column_gpio_pins_left_to_right":    fakePins,
	}
	component := config.Component{Attributes: fakeAttrMap}
	fsm, _ := New(context.Background(), fakeRobot, component, logger)
	matrix, err := fsm.Matrix(context.Background())
	test.That(t, err, test.ShouldBeNil)
	expectedMatrix := make([][]int, 4)
	for i := 0; i < len(expectedMatrix); i++ {
		expectedMatrix[i] = []int{1, 2, 3, 4}
	}
	test.That(t, matrix, test.ShouldResemble, expectedMatrix)
}
