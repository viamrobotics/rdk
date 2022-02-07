// Package oneaxis implements a one-axis gantry.
package oneaxis

import (
	"context"

	// for embedding model file.
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

const modelname = "oneaxis"

// AttrConfig is used for converting oneAxis config attributes.
type AttrConfig struct {
	Board           string   `json:"board"` // used to read limit switch pins and control motor with gpio pins
	Motor           string   `json:"motor"`
	LimitSwitchPins []string `json:"limit_pins"`
	LimitPinEnabled bool     `json:"limit_pin_enabled"`
	Axes            []bool   `json:"axes"`
	LengthMm        float64  `json:"length_mm"`
	ReductionRatio  float64  `json:"reduction_ratio"`
	GantryRPM       float64  `json:"gantry_rpm"`
}

//go:embed oneaxis-kinematics.json
var oneaxismodel []byte

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if config.Board == "" {
		return utils.NewConfigValidationError(path, errors.New("cannot find board for gantry"))
	}

	if len(config.Motor) == 0 {
		return utils.NewConfigValidationError(path, errors.New("cannot find motor for gantry"))
	}

	if config.LengthMm <= 0 {
		return utils.NewConfigValidationError(path, errors.New("each axis needs a non-zero and positive length"))
	}

	if len(config.LimitSwitchPins) == 0 {
		return utils.NewConfigValidationError(path, errors.New("each axis needs at least one limit switch pin"))
	}

	if len(config.LimitSwitchPins) == 1 && config.ReductionRatio == 0 {
		return utils.NewConfigValidationError(path,
			errors.New("gantry has one limit switch per axis, needs pulley radius to set position limits"),
		)
	}

	if len(config.Axes) == 0 {
		return utils.NewConfigValidationError(path, errors.New("axes not set")) // change after #471
	}

	// Need another way to test if LimitHigh is unset.
	// if config.LimitHigh {
	//		return utils.NewConfigValidationFieldRequiredError(path, "limitHigh")
	// }

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

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeGantry, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// Model returns the kinematics model of the oneaxis Gantry with all the frame information.
func Model() (referenceframe.Model, error) {
	return referenceframe.ParseJSON(oneaxismodel, "")
}

type oneAxis struct {
	name string

	board board.Board
	motor motor.Motor

	limitSwitchPins []string
	limitHigh       bool
	limitType       switchLimitType
	positionLimits  []float64

	lengthMm       float64
	reductionRatio float64
	rpm            float64

	model referenceframe.Model
	axes  []bool // TODO (rh) convert to r3.Vector once #471 is merged.

	logger golog.Logger
}

type switchLimitType string

const (
	switchLimitTypeEncoder = switchLimitType("encoder")
	switchLimitTypeOnePin  = switchLimitType("onePinOneLength")
	switchLimitTypetwoPin  = switchLimitType("twoPin")
)

// NewOneAxis creates a new one axis gantry.
func newOneAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	gconf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(gconf, config.ConvertedAttributes)
	}

	motor, ok := r.MotorByName(gconf.Motor)
	if !ok {
		return nil, errors.Errorf("cannot find motor named %v for gantry", config.Attributes.String("motor"))
	}

	board, ok := r.BoardByName(gconf.Board)
	if !ok {
		return nil, errors.New("cannot find board for gantry")
	}

	model, err := referenceframe.ParseJSON(oneaxismodel, "")
	if err != nil {
		return nil, err
	}

	gantry := &oneAxis{
		name:            config.Name,
		board:           board,
		motor:           motor,
		model:           model,
		logger:          logger,
		limitSwitchPins: gconf.LimitSwitchPins,
		limitHigh:       gconf.LimitPinEnabled,
		lengthMm:        gconf.LengthMm,
		reductionRatio:  gconf.ReductionRatio,
		rpm:             gconf.GantryRPM,
		axes:            gconf.Axes, // revisit axes def afer #471 (rh)
	}

	switch {
	case len(gantry.limitSwitchPins) == 1:
		gantry.limitType = switchLimitTypeOnePin
	case len(gantry.limitSwitchPins) == 2:
		gantry.limitType = switchLimitTypetwoPin
	case len(gantry.limitSwitchPins) == 0:
		gantry.limitType = switchLimitTypeEncoder
		return nil, errors.New("encoder currently not supported")
	default:
		np := len(gantry.limitSwitchPins)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", np)
	}

	if gantry.limitType == switchLimitTypeOnePin && gantry.reductionRatio <= 0 {
		return nil, errors.New("gantry with one limit switch per axis needs a pulley radius defined")
	}

	if err := gantry.Home(ctx); err != nil {
		return nil, err
	}

	return gantry, nil
}

// For Reviewers: Rename Home to SetZero?
func (g *oneAxis) Home(ctx context.Context) error {
	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	switch g.limitType {
	case switchLimitTypeOnePin:
		// limitIDs := []int{0, 1}
		err := g.homeOneLimSwitch(ctx) // , limitIDs)
		if err != nil {
			return err
		}
	case switchLimitTypetwoPin:
		// limitIDs := []int{0, 1}
		err := g.homeTwoLimSwitch(ctx) // , limitIDs)
		if err != nil {
			return err
		}
	case switchLimitTypeEncoder:
		err := g.homeEncoder(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *oneAxis) homeTwoLimSwitch(ctx context.Context) error {
	ok, err := g.motor.PositionSupported(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("gantry motor needs to support position")
	}

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
	err = g.motor.GoFor(ctx, float64(-1)*g.rpm, 2)
	if err != nil {
		return err
	}

	return nil
}

func (g *oneAxis) homeOneLimSwitch(ctx context.Context) error {
	ok, err := g.motor.PositionSupported(ctx)
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("gantry motor needs to support position")
	}

	// One pin always and only should go backwards.
	positionA, err := g.testLimit(ctx, true)
	if err != nil {
		return err
	}

	radius := g.reductionRatio
	stepsPerLength := g.lengthMm / (radius * 2 * math.Pi)

	positionB := positionA + stepsPerLength

	g.positionLimits = []float64{positionA, positionB}

	return nil
}

// Not yet implemented.
func (g *oneAxis) homeEncoder(ctx context.Context) error {
	return errors.New("encoder currently not supported")
}

func (g *oneAxis) testLimit(ctx context.Context, zero bool) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motor.Stop(ctx)
	})

	d := -1
	if !zero {
		d = 1
	}

	err := g.motor.GoFor(ctx, g.rpm, float64(d*10000))
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

	return g.motor.Position(ctx)
}

func (g *oneAxis) limitHit(ctx context.Context, zero bool) (bool, error) {
	offset := 0
	if !zero {
		offset = 1
	}
	pin := g.limitSwitchPins[offset]
	high, err := g.board.GetGPIO(ctx, pin)

	return high == g.limitHigh, err
}

// GetPosition returns the position in millimeters.
func (g *oneAxis) GetPosition(ctx context.Context) ([]float64, error) {
	pos, err := g.motor.Position(ctx)
	if err != nil {
		return []float64{}, err
	}

	theRange := g.positionLimits[1] - g.positionLimits[0]
	x := g.lengthMm * ((pos - g.positionLimits[0]) / theRange)

	limitAtZero, err := g.limitHit(ctx, true)
	if err != nil {
		return nil, err
	}

	if g.limitType == switchLimitTypetwoPin {
		limitAtOne, err := g.limitHit(ctx, false)
		if err != nil {
			return nil, err
		}
		g.logger.Debugf("%s CurrentPosition %.02f -> %.02f. limSwitch1: %t, limSwitch2: %t", g.name, x, limitAtZero, limitAtOne)
	}

	g.logger.Debugf("%s CurrentPosition %.02f -> %.02f. limSwitch1: %t", g.name, pos, x, limitAtZero)

	return []float64{x}, nil
}

// GetLengths returns the physical lengths of an axis of a Gantry.
func (g *oneAxis) GetLengths(ctx context.Context) ([]float64, error) {
	return []float64{g.lengthMm}, nil
}

// MoveToPosition moves along an axis using inputs in millimeters.
func (g *oneAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	if len(positions) != 1 {
		return fmt.Errorf("oneAxis gantry MoveToPosition needs 1 position, got: %v", len(positions))
	}

	if positions[0] < 0 || positions[0] > g.lengthMm {
		return fmt.Errorf("oneAxis gantry position out of range, got %.02f max is %.02f", positions[0], g.lengthMm)
	}

	theRange := g.positionLimits[1] - g.positionLimits[0]

	x := positions[0] / g.lengthMm
	x = g.positionLimits[0] + (x * theRange)

	g.logger.Debugf("oneAxis SetPosition %.2f -> %.2f", positions[0], x)

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
			return g.motor.GoFor(ctx, dir*g.rpm, 0.2*g.lengthMm)
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
	m := referenceframe.NewSimpleModel() //changeafter merge
	f, err := referenceframe.NewTranslationalFrame(g.name, g.axes, []referenceframe.Limit{{0, g.lengthMm}})
	if err != nil {
		panic(fmt.Errorf("error creating frame, should be impossible %w", err))
	}
	m.OrdTransforms = append(m.OrdTransforms, f)

	return m
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
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}
