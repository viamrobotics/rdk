package multiaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
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
		MoveToPositionFunc: func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
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
	_, err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "need at least one axis")

	fakecfg = &AttrConfig{SubAxes: []string{"singleaxis"}}
	_, err = fakecfg.Validate("path")
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
	err := fakemultiaxis.MoveToPosition(ctx, positions, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes, lengthsMm: []float64{1, 2, 3}}
	positions = []float64{1, 2, 3}
	err = fakemultiaxis.MoveToPosition(ctx, positions, nil)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes, lengthsMm: []float64{1, 2}}
	positions = []float64{1, 2}
	err = fakemultiaxis.MoveToPosition(ctx, positions, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fakemultiaxis := &multiAxis{}
	err := fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{subAxes: threeAxes, lengthsMm: []float64{1, 2, 3}}
	inputs = []referenceframe.Input{{Value: 1}, {Value: 2}, {Value: 3}}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{subAxes: twoAxes, lengthsMm: []float64{1, 2}}
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

func createComplexDeps() registry.Dependencies {
	position1 := []float64{6, 5}
	mAx1 := &inject.Gantry{
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return position1, nil
		},
		MoveToPositionFunc: func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
			if move, _ := extra["move"].(bool); move {
				position1[0] += pos[0]
				position1[1] += pos[1]
			}

			return nil
		},
		LengthsFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return []float64{100, 101}, nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
	}

	position2 := []float64{9, 8, 7}
	mAx2 := &inject.Gantry{
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return position2, nil
		},
		MoveToPositionFunc: func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
			if move, _ := extra["move"].(bool); move {
				position2[0] += pos[0]
				position2[1] += pos[1]
				position2[2] += pos[2]
			}
			return nil
		},
		LengthsFunc: func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return []float64{102, 103, 104}, nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
	}

	fakeMotor := &fm.Motor{}

	deps := make(registry.Dependencies)
	deps[gantry.Named("1")] = mAx1
	deps[gantry.Named("2")] = mAx2
	deps[motor.Named(fakeMotor.Name)] = fakeMotor
	return deps
}

func TestComplexMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := config.Component{
		Name: "complexGantry",
		ConvertedAttributes: &AttrConfig{
			SubAxes: []string{"1", "2"},
		},
	}
	deps := createComplexDeps()

	g, err := newMultiAxis(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("too many inputs", func(t *testing.T) {
		err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4, 5, 6}, nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("too few inputs", func(t *testing.T) {
		err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4}, nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("just right inputs", func(t *testing.T) {
		pos, err := g.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldResemble, []float64{6, 5, 9, 8, 7})
	})

	t.Run(
		"test that multiaxis moves and each subaxes moves correctly",
		func(t *testing.T) {
			extra := map[string]interface{}{"move": true}
			err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4, 5}, extra)
			test.That(t, err, test.ShouldBeNil)

			pos, err := g.Position(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldNotResemble, []float64{6, 5, 9, 8, 7})

			// This section tests out that each subaxes has moved, and moved the correct amount
			// according to it's input lengths
			currG, ok := g.(*multiAxis)
			test.That(t, ok, test.ShouldBeTrue)

			// This loop mimics the loop in MoveToposition to check that the correct
			// positions are sent to each subaxis
			idx := 0
			for _, subAx := range currG.subAxes {
				lengths, err := subAx.Lengths(ctx, nil)
				test.That(t, err, test.ShouldBeNil)

				subAxPos, err := subAx.Position(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(subAxPos), test.ShouldEqual, len(lengths))

				test.That(t, subAxPos, test.ShouldResemble, pos[idx:idx+len(lengths)])
				idx += len(lengths)
			}
		})
}
