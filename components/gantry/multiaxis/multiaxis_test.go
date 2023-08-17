package multiaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	fm "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func createFakeOneaAxis(length float64, positions []float64) *inject.Gantry {
	fakesingleaxis := inject.NewGantry("fake")
	fakesingleaxis.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return positions, nil
	}
	fakesingleaxis.MoveToPositionFunc = func(ctx context.Context, pos, speed []float64, extra map[string]interface{}) error {
		return nil
	}
	fakesingleaxis.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{length}, nil
	}
	fakesingleaxis.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}
	fakesingleaxis.HomeFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, nil
	}
	fakesingleaxis.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	fakesingleaxis.ModelFrameFunc = func() referenceframe.Model {
		return nil
	}
	return fakesingleaxis
}

func createFakeDeps() resource.Dependencies {
	fakeGantry1 := inject.NewGantry("1")
	fakeGantry1.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{1}, nil
	}
	fakeGantry2 := inject.NewGantry("2")
	fakeGantry2.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{1}, nil
	}
	fakeGantry3 := inject.NewGantry("3")
	fakeGantry3.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{1}, nil
	}
	fakeMotor := &fm.Motor{
		Named: motor.Named("fm1").AsNamed(),
	}

	deps := make(resource.Dependencies)
	deps[fakeGantry1.Name()] = fakeGantry1
	deps[fakeGantry2.Name()] = fakeGantry2
	deps[fakeGantry3.Name()] = fakeGantry3
	deps[fakeMotor.Name()] = fakeMotor
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
	fakecfg := &Config{SubAxes: []string{}}
	_, err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "need at least one axis")

	fakecfg = &Config{SubAxes: []string{"singleaxis"}}
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	deps := createFakeDeps()

	fakeMultAxcfg := resource.Config{
		Name: "gantry",
		ConvertedAttributes: &Config{
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

	fakeMultAxcfg = resource.Config{
		Name: "gantry",
		Attributes: rutils.AttributeMap{
			"subaxes_list": []string{},
		},
	}
	_, err = newMultiAxis(ctx, deps, fakeMultAxcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	positions := []float64{}
	speeds := []float64{}

	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	err := fakemultiaxis.MoveToPosition(ctx, positions, speeds, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{
		subAxes:   threeAxes,
		lengthsMm: []float64{1, 2, 3},
		opMgr:     operation.NewSingleOperationManager(),
	}
	positions = []float64{1, 2, 3}
	speeds = []float64{100, 200, 300}
	err = fakemultiaxis.MoveToPosition(ctx, positions, speeds, nil)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{
		subAxes:   twoAxes,
		lengthsMm: []float64{1, 2},
		opMgr:     operation.NewSingleOperationManager(),
	}
	positions = []float64{1, 2}
	speeds = []float64{100, 200}
	err = fakemultiaxis.MoveToPosition(ctx, positions, speeds, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	err := fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{
		subAxes:   threeAxes,
		lengthsMm: []float64{1, 2, 3},
		opMgr:     operation.NewSingleOperationManager(),
	}
	inputs = []referenceframe.Input{{Value: 1}, {Value: 2}, {Value: 3}}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)

	fakemultiaxis = &multiAxis{
		subAxes:   twoAxes,
		lengthsMm: []float64{1, 2},
		opMgr:     operation.NewSingleOperationManager(),
	}
	inputs = []referenceframe.Input{{Value: 1}, {Value: 2}}
	err = fakemultiaxis.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}

func TestPosition(t *testing.T) {
	ctx := context.Background()

	fakemultiaxis := &multiAxis{
		subAxes: threeAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	pos, err := fakemultiaxis.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5, 9})

	fakemultiaxis = &multiAxis{
		subAxes: twoAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	pos, err = fakemultiaxis.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, []float64{1, 5})
}

func TestLengths(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	lengths, err := fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{})

	fakemultiaxis = &multiAxis{
		subAxes: threeAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	lengths, err = fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{1, 2, 3})

	fakemultiaxis = &multiAxis{
		subAxes: twoAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}

	lengths, err = fakemultiaxis.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{5, 6})
}

func TestHome(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	homed, err := fakemultiaxis.Home(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, homed, test.ShouldBeTrue)

	fakemultiaxis = &multiAxis{
		subAxes: threeAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	homed, err = fakemultiaxis.Home(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, homed, test.ShouldBeTrue)

	fakemultiaxis = &multiAxis{
		subAxes: twoAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}

	homed, err = fakemultiaxis.Home(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, homed, test.ShouldBeTrue)
}

func TestStop(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)

	fakemultiaxis = &multiAxis{
		subAxes: threeAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)

	fakemultiaxis = &multiAxis{
		subAxes: twoAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	test.That(t, fakemultiaxis.Stop(ctx, nil), test.ShouldBeNil)
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	fakemultiaxis := &multiAxis{
		opMgr: operation.NewSingleOperationManager(),
	}
	inputs, err := fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input(nil))

	fakemultiaxis = &multiAxis{
		subAxes: threeAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{{Value: 1}, {Value: 5}, {Value: 9}})

	fakemultiaxis = &multiAxis{
		subAxes: twoAxes,
		opMgr:   operation.NewSingleOperationManager(),
	}
	inputs, err = fakemultiaxis.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, []referenceframe.Input{{Value: 1}, {Value: 5}})
}

func TestModelFrame(t *testing.T) {
	fakemultiaxis := &multiAxis{
		Named:     gantry.Named("foo").AsNamed(),
		subAxes:   twoAxes,
		lengthsMm: []float64{1, 1},
		opMgr:     operation.NewSingleOperationManager(),
	}
	model := fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldNotBeNil)

	fakemultiaxis = &multiAxis{
		Named:     gantry.Named("foo").AsNamed(),
		subAxes:   threeAxes,
		lengthsMm: []float64{1, 1, 1},
		opMgr:     operation.NewSingleOperationManager(),
	}
	model = fakemultiaxis.ModelFrame()
	test.That(t, model, test.ShouldNotBeNil)
}

func createComplexDeps() resource.Dependencies {
	position1 := []float64{6, 5}
	mAx1 := inject.NewGantry("1")
	mAx1.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return position1, nil
	}
	mAx1.MoveToPositionFunc = func(ctx context.Context, pos, speeds []float64, extra map[string]interface{}) error {
		if move, _ := extra["move"].(bool); move {
			position1[0] += pos[0]
			position1[1] += pos[1]
		}

		return nil
	}
	mAx1.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{100, 101}, nil
	}
	mAx1.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}

	position2 := []float64{9, 8, 7}
	mAx2 := inject.NewGantry("2")
	mAx2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return position2, nil
	}
	mAx2.MoveToPositionFunc = func(ctx context.Context, pos, speeds []float64, extra map[string]interface{}) error {
		if move, _ := extra["move"].(bool); move {
			position2[0] += pos[0]
			position2[1] += pos[1]
			position2[2] += pos[2]
		}
		return nil
	}
	mAx2.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{102, 103, 104}, nil
	}
	mAx2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}

	fakeMotor := &fm.Motor{
		Named: motor.Named("foo").AsNamed(),
	}

	deps := make(resource.Dependencies)
	deps[mAx1.Name()] = mAx1
	deps[mAx2.Name()] = mAx2
	deps[fakeMotor.Name()] = fakeMotor
	return deps
}

func TestComplexMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := resource.Config{
		Name: "complexGantry",
		ConvertedAttributes: &Config{
			SubAxes: []string{"1", "2"},
		},
	}
	deps := createComplexDeps()

	g, err := newMultiAxis(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("too many inputs", func(t *testing.T) {
		err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4, 5, 6}, []float64{100, 200, 300, 300, 200, 100}, nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("too few inputs", func(t *testing.T) {
		err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4}, []float64{100, 200, 300, 300, 200, 100}, nil)
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
			err = g.MoveToPosition(ctx, []float64{1, 2, 3, 4, 5}, []float64{100, 200, 300, 200, 100}, extra)
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
