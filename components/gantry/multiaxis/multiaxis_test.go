package multiaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	fm "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils/inject"
)

func createFakeOneaAxis(length float64, positions []float64) *inject.Gantry {
	fakeoneaxis := &inject.Gantry{
		LocalGantry: nil,
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return positions, nil
		},
		MoveToPositionFunc: func(ctx context.Context, positions []float64, worldState *commonpb.WorldState, extra map[string]interface{}) error {
			return nil
		},
		LengthsFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return []float64{length}, nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
		ModelFrameFunc: func() referenceframe.Model {
			return nil
		},
	}
	return fakeoneaxis
}

func createFakeDeps() registry.Dependencies {
	fakeGantry := &inject.Gantry{
		LengthsFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return []float64{1}, nil
		},
	}
	fakeMotor := &fm.Motor{}

	deps := make(registry.Dependencies)
	deps[gantry.Named("1")] = fakeGantry
	deps[gantry.Named("2")] = fakeGantry
	deps[gantry.Named("3")] = fakeGantry
	deps[motor.Named(fakeMotor.Name)] = fakeMotor
	return deps
}

var threeAxes = []gantry.Gantry{
	createFakeOneaAxis(1, []float64{1}),
	createFakeOneaAxis(2, []float64{5}),
	createFakeOneaAxis(3, []float64{9}),
}

var twoAxes = []gantry.Gantry{
	createFakeOneaAxis(5, []float64{1}),
	createFakeOneaAxis(6, []float64{5}),
}

func TestValidate(t *testing.T) {
	fakecfg := &AttrConfig{SubAxes: []string{}}
	err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "need at least one axis")

	fakecfg = &AttrConfig{SubAxes: []string{"singleaxis"}}
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	deps := createFakeDeps()

	fakeMultAxcfg := config.Component{
		Name: "gantry",
		ConvertedAttributes: &AttrConfig{
			SubAxes: []string{"1", "2", "3"},
		},
	}
	fmag, err := newMultiAxis(ctx, deps, fakeMultAxcfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fmag, test.ShouldNotBeNil)
	fakemulax, ok := fmag.(*multiAxis)
	test.That(t, ok, test.ShouldBeTrue)
	lenfloat := []float64{1, 1, 1}
	test.That(t, fakemulax.lengthsMm, test.ShouldResemble, lenfloat)

	fakeMultAxcfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"subaxes_list": []string{},
		},
	}
	_, err = newMultiAxis(ctx, deps, fakeMultAxcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	positions := []float64{}

	fakemultiaxis := &multiAxis{}
	err := fakemultiaxis.MoveToPosition(ctx, positions, &commonpb.WorldState{}, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	positions = []float64{1, 2, 3}
	err = fakemultiaxis.MoveToPosition(ctx, positions, &commonpb.WorldState{}, nil)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	positions = []float64{1, 2}
	err = fakemultiaxis.MoveToPosition(ctx, positions, &commonpb.WorldState{}, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fakemultiaxis := &multiAxis{}
	err := fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	inputs = []referenceframe.Input{{Value: 1}, {Value: 2}, {Value: 3}}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	inputs = []referenceframe.Input{{Value: 1}, {Value: 2}}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}

func TestPosition(t *testing.T) {
	ctx := context.Background()

	fakemultiaxis := &multiAxis{subAxes: threeAxes}
	pos, err := fakemultiaxis.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5, 9})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	pos, err = fakemultiaxis.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5})
}

func TestLengths(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{}
	lengths, err := fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{})

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	lengths, err = fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{1, 2, 3})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}

	lengths, err = fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{5, 6})
}

func TestStop(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{}
	inputs, err := fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input(nil))

	fakemultiaxis = &multiAxis{subAxes: threeAxes}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{{Value: 1}, {Value: 5}, {Value: 9}})

	fakemultiaxis = &multiAxis{subAxes: twoAxes}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{{Value: 1}, {Value: 5}})
}

func TestModelFrame(t *testing.T) {
	fakemultiaxis := &multiAxis{subAxes: twoAxes, lengthsMm: []float64{1, 1}, name: "test2Axgood"}
	model := fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes, lengthsMm: []float64{1, 1, 1}, name: "test3Axgood"}
	model = fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldNotBeNil)
}
