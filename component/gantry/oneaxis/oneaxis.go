// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const modelname = "oneaxis"

// AttrConfig is used for converting oneAxis config attributes.
type AttrConfig struct {
	Board           string                    `json:"board,omitempty"` // used to read limit switch pins and control motor with gpio pins
	Motor           string                    `json:"motor"`
	LimitSwitchPins []string                  `json:"limit_pins,omitempty"`
	LimitPinEnabled *bool                     `json:"limit_pin_enabled_high,omitempty"`
	LengthMm        float64                   `json:"length_mm"`
	MmPerRevolution float64                   `json:"mm_per_revolution,omitempty"`
	GantryRPM       float64                   `json:"gantry_rpm,omitempty"`
	Axis            spatial.TranslationConfig `json:"axis"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if len(config.Motor) == 0 {
		return utils.NewConfigValidationError(path, errors.New("cannot find motor for gantry"))
	}

	if config.LengthMm <= 0 {
		return utils.NewConfigValidationError(path, errors.New("each axis needs a non-zero and positive length"))
	}

	if len(config.LimitSwitchPins) == 0 && config.Board != "" {
		return utils.NewConfigValidationError(path, errors.New("gantry with encoders have to assign boards or controllers to motors"))
	}

	if config.Board == "" && len(config.LimitSwitchPins) > 0 {
		return utils.NewConfigValidationError(path, errors.New("cannot find board for gantry"))
	}

	if len(config.LimitSwitchPins) == 1 && config.MmPerRevolution == 0 {
		return utils.NewConfigValidationError(path,
			errors.New("gantry has one limit switch per axis, needs pulley radius to set position limits"))
	}

	if len(config.LimitSwitchPins) > 0 && config.LimitPinEnabled == nil {
		return utils.NewConfigValidationError(path, errors.New("limit pin enabled muist be set to true or false"))
	}

	if config.Axis.X == 0 && config.Axis.Y == 0 && config.Axis.Z == 0 {
		return utils.NewConfigValidationError(path, errors.New("gantry axis undefined, need one translational axis"))
	}

	if config.Axis.X == 1 && config.Axis.Y == 1 ||
		config.Axis.X == 1 && config.Axis.Z == 1 || config.Axis.Y == 1 && config.Axis.Z == 1 {
		return utils.NewConfigValidationError(path, errors.New("only one translational axis of movement allowed for single axis gantry"))
	}

	return nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, modelname, registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newOneAxis(ctx, r, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(gantry.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

type oneAxis struct {
	generic.Unimplemented
	name string

	board board.Board
	motor motor.Motor

	limitSwitchPins []string
	limitHigh       bool
	limitType       limitType
	positionLimits  []float64

	lengthMm        float64
	mmPerRevolution float64
	rpm             float64

	model                 referenceframe.Model
	axis                  r3.Vector
	axisOrientationOffset *spatial.OrientationVector
	axisTranslationOffset r3.Vector

	logger golog.Logger
	opMgr  operation.SingleOperationManager
}

type limitType string

const (
	limitEncoder = limitType("encoder")
	limitOnePin  = limitType("onePinOneLength")
	limitTwoPin  = limitType("twoPin")
)

// NewOneAxis creates a new one axis gantry.
func newOneAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(conf, config.ConvertedAttributes)
	}

	_motor, err := motor.FromRobot(r, conf.Motor)
	if err != nil {
		return nil, err
	}
	features, err := _motor.GetFeatures(ctx)
	if err != nil {
		return nil, err
	}
	ok = features[motor.PositionReporting]
	if !ok {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, conf.Motor)
	}

	oAx := &oneAxis{
		name:            config.Name,
		motor:           _motor,
		logger:          logger,
		limitSwitchPins: conf.LimitSwitchPins,
		lengthMm:        conf.LengthMm,
		mmPerRevolution: conf.MmPerRevolution,
		rpm:             conf.GantryRPM,
		axis:            r3.Vector(conf.Axis),
	}

	switch len(oAx.limitSwitchPins) {
	case 1:
		oAx.limitType = limitOnePin
		if oAx.mmPerRevolution <= 0 {
			return nil, errors.New("gantry with one limit switch per axis needs a mm_per_length ratio defined")
		}

		board, err := board.FromRobot(r, conf.Board)
		if err != nil {
			return nil, err
		}
		oAx.board = board

		PinEnable := *conf.LimitPinEnabled
		oAx.limitHigh = PinEnable

	case 2:
		oAx.limitType = limitTwoPin
		board, err := board.FromRobot(r, conf.Board)
		if err != nil {
			return nil, err
		}
		oAx.board = board

		PinEnable := *conf.LimitPinEnabled
		oAx.limitHigh = PinEnable

	case 0:
		oAx.limitType = limitEncoder
	default:
		np := len(oAx.limitSwitchPins)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", np)
	}

	if err := oAx.Home(ctx); err != nil {
		return nil, err
	}

	return oAx, nil
}

func (g *oneAxis) Home(ctx context.Context) error {
	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	switch g.limitType {
	// An axis with one limit switch will go till it hits the limit switch, encode that position as the
	// zero position of the one-axis, and adds a second position limit based on the steps per length.
	case limitOnePin:
		err := g.homeOneLimSwitch(ctx)
		if err != nil {
			return err
		}
	// An axis with two limit switches will go till it hits the first limit switch, encode that position as the
	// zero position of the one-axis, then go till it hits the second limit switch, then encode that position as the
	// at-length position of the one-axis.
	case limitTwoPin:
		err := g.homeTwoLimSwitch(ctx)
		if err != nil {
			return err
		}
	// An axis with an encoder will encode the
	case limitEncoder:
		err := g.homeEncoder(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *oneAxis) homeTwoLimSwitch(ctx context.Context) error {
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	positionB, err := g.testLimit(ctx, false)
	if err != nil {
		return err
	}

	g.logger.Debugf("positionA: %0.2f positionB: %0.2f", positionA, positionB)

	g.positionLimits = []float64{positionA, positionB}

	// Go backwards so limit stops are not hit.
	x := g.rotationalToLinear(0.8 * g.lengthMm)
	err = g.motor.GoTo(ctx, g.rpm, x)
	if err != nil {
		return err
	}
	return nil
}

func (g *oneAxis) homeOneLimSwitch(ctx context.Context) error {
	// One pin always and only should go backwards.
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	revPerLength := g.lengthMm / g.mmPerRevolution
	positionB := positionA + revPerLength

	g.positionLimits = []float64{positionA, positionB}

	return nil
}

func (g *oneAxis) homeEncoder(ctx context.Context) error {
	revPerLength := g.lengthMm / g.mmPerRevolution

	positionA, err := g.motor.GetPosition(ctx)
	if err != nil {
		return err
	}

	positionB := positionA + revPerLength

	g.positionLimits = []float64{positionA, positionB}
	return nil
}

func (g *oneAxis) rotationalToLinear(positions float64) float64 {
	theRange := g.positionLimits[1] - g.positionLimits[0]
	x := positions / g.lengthMm
	x = g.positionLimits[0] + (x * theRange)
	g.logger.Debugf("oneAxis SetPosition %.2f -> %.2f", positions, x)

	return x
}

func (g *oneAxis) testLimit(ctx context.Context, zero bool) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motor.Stop(ctx)
	})

	d := -1.0
	if !zero {
		d *= -1
	}

	err := g.motor.GoFor(ctx, d*g.rpm, 0)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, zero)
		if err != nil {
			return 0, err
		}
		if hit {
			err = g.motor.Stop(ctx)
			if err != nil {
				return 0, err
			}
			break
		}

		elapsed := start.Sub(start)
		if elapsed > (time.Second * 15) {
			return 0, errors.New("gantry timed out testing limit")
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*10) {
			return 0, ctx.Err()
		}
	}

	return g.motor.GetPosition(ctx)
}

func (g *oneAxis) limitHit(ctx context.Context, zero bool) (bool, error) {
	offset := 0
	if !zero {
		offset = 1
	}
	pin, err := g.board.GPIOPinByName(g.limitSwitchPins[offset])
	if err != nil {
		return false, err
	}
	high, err := pin.Get(ctx)

	return high == g.limitHigh, err
}

// GetPosition returns the position in millimeters.
func (g *oneAxis) GetPosition(ctx context.Context) ([]float64, error) {
	pos, err := g.motor.GetPosition(ctx)
	if err != nil {
		return []float64{}, err
	}

	theRange := g.positionLimits[1] - g.positionLimits[0]
	x := g.lengthMm * ((pos - g.positionLimits[0]) / theRange)

	return []float64{x}, nil
}

// GetLengths returns the physical lengths of an axis of a Gantry.
func (g *oneAxis) GetLengths(ctx context.Context) ([]float64, error) {
	return []float64{g.lengthMm}, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *oneAxis) MoveToPosition(ctx context.Context, positions []float64, worldState *commonpb.WorldState) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()

	if len(positions) != 1 {
		return fmt.Errorf("oneAxis gantry MoveToPosition needs 1 position, got: %v", len(positions))
	}

	if positions[0] < 0 || positions[0] > g.lengthMm {
		return fmt.Errorf("oneAxis gantry position out of range, got %.02f max is %.02f", positions[0], g.lengthMm)
	}

	x := g.rotationalToLinear(positions[0])
	// Limit switch errors that stop the motors.
	// Currently needs to be moved by underlying gantry motor.
	hit, err := g.limitHit(ctx, true)
	if err != nil {
		return err
	}

	// Hits backwards limit switch, goes in forwards direction for two revolutions
	if hit {
		if x < g.positionLimits[0] {
			dir := float64(1)
			return g.motor.GoFor(ctx, dir*g.rpm, 2)
		}
		return g.motor.Stop(ctx)
	}

	// Hits forward limit switch, goes in backwards direction for two revolutions
	hit, err = g.limitHit(ctx, false)
	if err != nil {
		return err
	}
	if hit {
		if x > g.positionLimits[1] {
			dir := float64(-1)
			return g.motor.GoFor(ctx, dir*g.rpm, 2)
		}
		return g.motor.Stop(ctx)
	}

	err = g.motor.GoTo(ctx, g.rpm, x)
	if err != nil {
		return err
	}
	return nil
}

//  ModelFrame returns the frame model of the Gantry.
func (g *oneAxis) ModelFrame() referenceframe.Model {
	if g.model == nil {
		var errs error
		m := referenceframe.NewSimpleModel()

		f, err := referenceframe.NewStaticFrame(g.name,
			spatial.NewPoseFromOrientationVector(g.axisTranslationOffset, g.axisOrientationOffset))
		errs = multierr.Combine(errs, err)
		m.OrdTransforms = append(m.OrdTransforms, f)

		f, err = referenceframe.NewTranslationalFrame(g.name, g.axis, referenceframe.Limit{Min: 0, Max: g.lengthMm})
		errs = multierr.Combine(errs, err)

		if errs != nil {
			g.logger.Error(errs)
			return nil
		}

		m.OrdTransforms = append(m.OrdTransforms, f)
		g.model = m
	}
	return g.model
}

// CurrentInputs returns the current inputs of the Gantry frame.
func (g *oneAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

// GoToInputs moves the gantry to a goal position in the Gantry frame.
func (g *oneAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), &commonpb.WorldState{})
}
