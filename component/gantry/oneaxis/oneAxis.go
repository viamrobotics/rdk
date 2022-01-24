// Package oneAxis implements a one-axis gantry.
package oneAxis

import (
	"context"
	"fmt"
	"math"
	"time"

	// for embedding model file.
	_ "embed"

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
)

const modelname = "oneAxis"

// OneAxisConfig is used for converting config attributes.
type OneAxisConfig struct {
	Board           string  `json:"board"` // used to read limit switch pins and control motor with gpio pins
	LimitSwitchPins string  `json:"limitPins"`
	LimitHigh       string  `json:"limitHigh"`
	MotorList       string  `json:"motor"`
	Axes            []bool  `json:"axes"`
	Length_mm       float64 `json:"length_mm"`
	PulleyR_mm      string  `json:"pulleyRadius_mm"`
	RPM             float64 `json:"rpm"`
}

//go:embed oneAxis_kinematics.json
var oneaxismodel []byte

func (config *OneAxisConfig) Validate(path string) error {
	if config.Board == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if len(config.MotorList) == 0 {
		return utils.NewConfigValidationError(path, errors.New("cannot find motors for gantry"))
	}

	if config.Length_mm <= 0 {
		return utils.NewConfigValidationError(path, errors.New("each axis needs a non-zero and positive length"))
	}

	if config.LimitHigh == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "limitHigh")
	}

	if len(config.Axes) != 3 {
		return utils.NewConfigValidationError(path, errors.New(""))
	}

	return nil
}

func init() {
	registry.RegisterComponent(gantry.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewOneAxis(ctx, r, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeGantry, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf OneAxisConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&OneAxisConfig{})
}

// oneAxisModel() returns the kinematics model of the oneAxisGantry with all the frame information.
func OneAxisModel() (referenceframe.Model, error) {
	return referenceframe.ParseJSON(oneaxismodel, "")
}

type oneAxis struct {
	name string

	board           board.Board
	limitSwitchPins []string
	limitHigh       bool
	motor           motor.Motor
	axes            []bool
	length_mm       float64
	pulleyR_mm      float64
	rpm             float64

	limitType      switchLimitType
	positionLimits []float64

	model referenceframe.Model

	logger golog.Logger
}

type switchLimitType string

const (
	switchLimitTypeEncoder = switchLimitType("encoder")
	switchLimitTypeOnePin  = switchLimitType("onePinOneLength")
	switchLimitTypetwoPin  = switchLimitType("twoPin")
)

// NewOneAxis creates a new one axis gantry.
func NewOneAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	gantry := &oneAxis{
		name:   config.Name,
		logger: logger,
	}

	var ok bool

	gantry.motor, ok = r.MotorByName(config.Attributes.String("motor"))
	if !ok {
		return nil, errors.Errorf("cannot find motor named %v for gantry", config.Attributes.String("motor"))
	}

	gantry.board, ok = r.BoardByName(config.Attributes.String("board"))
	if !ok {
		return nil, errors.New("cannot find board for gantry")
	}

	gantry.limitSwitchPins = config.Attributes.StringSlice("limitSwitchPins")
	if len(gantry.limitSwitchPins) == 1 {
		gantry.limitType = switchLimitTypeOnePin
	} else if len(gantry.limitSwitchPins) == 2 {
		gantry.limitType = switchLimitTypetwoPin
	} else if len(gantry.limitSwitchPins) == 0 {
		gantry.limitType = switchLimitTypeEncoder
		// encoder not supported currently.
	} else {
		np := len(gantry.limitSwitchPins)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins", np)
	}

	gantry.limitHigh = config.Attributes.Bool("limitHigh", true)

	gantry.length_mm = config.Attributes.Float64("length_mm", 0.0)
	if gantry.length_mm <= 0 {
		return nil, errors.New("gantry length has to be >= 0")
	}

	gantry.rpm = config.Attributes.Float64("rpm", 10.0)

	var err error
	gantry.model, err = referenceframe.ParseJSON(oneaxismodel, "")
	if err != nil {
		return nil, err
	}

	if err := gantry.init(ctx); err != nil {
		return nil, err
	}

	return gantry, nil
}

func (g *oneAxis) init(ctx context.Context) error {
	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	switch g.limitType {
	case switchLimitTypeOnePin:
		// limitIDs := []int{0, 1}
		err := g.homeOneLimSwitch(ctx) //, limitIDs)
		if err != nil {
			return err
		}
	case switchLimitTypetwoPin:
		// limitIDs := []int{0, 1}
		err := g.homeTwoLimSwitch(ctx) //, limitIDs)
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
	g.motor.GoFor(ctx, float64(-1)*g.rpm, 2)

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

	radius := g.pulleyR_mm
	stepsPerLength := g.length_mm / (radius * 2 * math.Pi)

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

		// want to test out elapse error
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
<<<<<<< HEAD
	high, err := g.limitBoard.GetGPIO(ctx, pin)
=======
	high, err := g.board.GetGPIO(ctx, pin)
>>>>>>> 92e44b88 (moved multiAxis tests to oneAxis tests)

	return high == g.limitHigh, err
}

// Position returns the position in meters.
func (g *oneAxis) GetPosition(ctx context.Context) ([]float64, error) {
	pos, err := g.motor.Position(ctx)
	if err != nil {
		return []float64{}, err
	}

	theRange := g.positionLimits[1] - g.positionLimits[0]
	x := g.length_mm * ((pos - g.positionLimits[0]) / theRange)

	limitAtZero, err := g.limitHit(ctx, true)

	limitAtOne, err := g.limitHit(ctx, false)

	// Prints out Motor position, Gantry position along length, state of tlimit switches.
	g.logger.Debugf("oneAxis CurrentPosition %.02f -> %.02f. limSwitch1: %t, limSwitch2: %t", pos, x, limitAtZero, limitAtOne)

	return []float64{x}, nil
}

<<<<<<< HEAD:component/gantry/simple/one_axis.go
func (g *oneAxis) GetLengths(ctx context.Context) ([]float64, error) {
	return []float64{g.lengthMeters}, nil
=======
func (g *oneAxis) Lengths(ctx context.Context) ([]float64, error) {
	return []float64{g.length_mm}, nil
>>>>>>> bef53fc4 (renamed and reorganized files):component/gantry/oneAxis/oneAxis.go
}

// Position is in meters.
func (g *oneAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	if len(positions) != 1 {
		return fmt.Errorf("oneAxis gantry MoveToPosition needs 1 position, got: %.02f", positions)
	}

	if positions[0] < 0 || positions[0] > g.length_mm {
		return fmt.Errorf("oneAxis gantry position out of range, got %.02f max is %.02f", positions[0], g.length_mm)
	}

	theRange := g.positionLimits[1] - g.positionLimits[0]

	x := positions[0] / g.length_mm
	x = g.positionLimits[0] + (x * theRange)

	g.logger.Debugf("oneAxis SetPosition %.2f -> %.2f", positions[0], x)

	// Limit switch errors that stop the motors.
	// Currently needs to be moved by underlying gantry motor.
	hit, err := g.limitHit(ctx, true)

	// Hits backwards limit switch, goes in forwards direction for two revolutions
	if hit {
		if x < g.positionLimits[0] {
			dir := float64(1)
			return g.motor.GoFor(ctx, dir*g.rpm, 2)
		} else {
			return g.motor.Stop(ctx)
		}
	}

	// Hits forward limit switch, goes in backwards direction for two revolutions
	hit, err = g.limitHit(ctx, false)
	if hit {
		if x > g.positionLimits[1] {
			dir := float64(-1)
			return g.motor.GoFor(ctx, dir*g.rpm, 2)
		} else {
			return g.motor.Stop(ctx)
		}
	}

	err = g.motor.GoTo(ctx, g.rpm, x)
	if err != nil {
		return err
	}
	return nil
}

func (g *oneAxis) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
	f, err := referenceframe.NewTranslationalFrame(
		g.name,
		[]bool{true},
		[]referenceframe.Limit{{0, g.length_mm}},
	)
	if err != nil {
		panic(fmt.Errorf("error creating frame, should be impossible %w", err))
	}
	m.OrdTransforms = append(m.OrdTransforms, f)

	return m
}

func (g *oneAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

func (g *oneAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}
