package multiaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/motor"
	fm "go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func createFakeOneaAxis(length float64, positions []float64) *inject.Gantry {
	fakeoneaxis := &inject.Gantry{
		Gantry: nil,
		GetPositionFunc: func(ctx context.Context) ([]float64, error) {
			return positions, nil
		},
		MoveToPositionFunc: func(ctx context.Context, positions []float64) error {
			return nil
		},
		GetLengthsFunc: func(ctx context.Context) ([]float64, error) {
			return []float64{length}, nil
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
	}
	return fakeoneaxis
}

func createFakeRobot() *inject.Robot {
	fakerobot := &inject.Robot{}

	fakerobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return &fm.Motor{PositionSupportedFunc: true, GoForfunc: true}, true
	}

	fakerobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &inject.Board{GetGPIOFunc: func(ctx context.Context, pin string) (bool, error) { return true, nil }}, true
	}

	// unsure if right way to test if ResourceByName is working correctly
	fakerobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return &inject.Gantry{GetLengthsFunc: func(ctx context.Context) ([]float64, error) { return []float64{1, 0, 0}, nil }}, true
	}
	return fakerobot
}

var threeAxes = []gantry.Gantry{
	createFakeOneaAxis(1, []float64{1, 2, 3}),
	createFakeOneaAxis(2, []float64{4, 5, 6}),
	createFakeOneaAxis(3, []float64{7, 8, 9}),
}

var twoAxes = []gantry.Gantry{
	createFakeOneaAxis(5, []float64{1, 2, 3}),
	createFakeOneaAxis(6, []float64{4, 5, 6}),
}

func TestNewMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakeRobot := createFakeRobot()

	fakeMultAxcfg := config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"subaxes_list": []string{"1", "2", "3"},
		},
	}

	_, err := NewMultiAxis(ctx, fakeRobot, fakeMultAxcfg, logger)
	test.That(t, err, test.ShouldBeNil)

	fakeMultAxcfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"subaxes_list": []string{},
		},
	}

	_, err = NewMultiAxis(ctx, fakeRobot, fakeMultAxcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	positions := []float64{}

	fakemultiaxis := &multiAxis{}
	err := fakemultiaxis.MoveToPosition(ctx, positions)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	positions = []float64{1, 2, 3}
	err = fakemultiaxis.MoveToPosition(ctx, positions)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	positions = []float64{1, 2}
	err = fakemultiaxis.MoveToPosition(ctx, positions)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fakemultiaxis := &multiAxis{}
	err := fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPosition(t *testing.T) {
	ctx := context.Background()

	fakemultiaxis := &multiAxis{subAxes: threeAxes}
	pos, err := fakemultiaxis.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5, 9})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	pos, err = fakemultiaxis.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5})
}

func TestGetLengths(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{}
	lengths, err := fakemultiaxis.GetLengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{})

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	lengths, err = fakemultiaxis.GetLengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{1, 2, 3})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}

	lengths, err = fakemultiaxis.GetLengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{5, 6})
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{}
	inputs, err := fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{})

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{})
}

/*
func TestModelFrame(t *testing.T) {
	fakemultiaxis := &multiAxis{}
	model := fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldResemble, nil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	model = fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldResemble, []referenceframe.Input{})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	model = fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldResemble, []referenceframe.Input{})
}
*/
