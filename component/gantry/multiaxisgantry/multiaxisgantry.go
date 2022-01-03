package multiaxisgantry

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(gantry.Subtype, "multiaxis", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMultiAxis(ctx, r, config, logger)
		},
	})
}

type switchLimitType string

const (
	switchLimiTypeEncoder = switchLimitType("encoder")
	switchLimitTypeOnePin = switchLimitType("onePin")
)

// NewMultiAxis creates a new-multi axis gantry.
func NewMultiAxis(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gantry.Gantry, error) {
	g := &multiAxis{
		logger:          logger,
		name:            config.Name,
		motorList:       []string{},
		motors:          []motor.Motor{},
		limitSwitchPins: []string{},
		limitBoard:      nil,
		limitHigh:       false,
		axes:            []bool{},
		limitType:       "",
		lengthMeters:    []float64{},
		rpm:             []float64{},
		positionLimits:  []float64{},
		pulleyR:         []float64{},
	}

	var ok bool

	// The multi axis gantry uses the motors to consturct the axes of the gantry
	// Logic considering # axes != to # of motors should be added if we need to use reference
	// frames in the future.
	g.motorList = config.Attributes.StringSlice("motors")
	if len(g.motorList) < 1 {
		return nil, errors.New("need at least 1 axis")
	} else if len(g.motorList) > 2 {
		return nil, errors.New("up to 3 axes supported")
	}

	for idx := range g.motorList {
		g.motors[idx], ok = r.MotorByName(g.motorList[idx])
	}
	if !ok {
		return nil, errors.New("cannot find motors for gantry")
	}

	g.limitBoard, ok = r.BoardByName(config.Attributes.String("limitBoard"))
	if !ok {
		return nil, errors.New("cannot find board for gantry")
	}

	g.limitHigh = config.Attributes.Bool("limitHigh", true)

	g.lengthMeters = config.Attributes.Float64Slice("lengthMeters")
	if len(g.lengthMeters) <= len(g.motorList) {
		return nil, errors.New("each axis needs a non-zero length")
	}

	g.pulleyR = config.Attributes.Float64Slice("pulleyR")
	if g.limitType == "onePinOneLength" && len(g.pulleyR) <= len(g.motorList) {
		return nil, errors.New("gantries that have one limit switch per axis require pulley radii to be specified")
	}

	for idx := range g.motorList {
		if g.lengthMeters[idx] <= 0 {
			return nil, errors.New("all axes must have non-zero length")
		}
	}

	g.limitSwitchPins = config.Attributes.StringSlice("limitPins")
	// 0 for encoder, 1 per axis for single limit pin per axis (Cheaper 3D printers).
	// and 2*number of axes for most gantries.
	if len(g.limitSwitchPins) == len(g.motorList) {
		g.limitType = "onePinOneLength"
	} else if len(g.limitSwitchPins) == 2*len(g.motorList) {
		g.limitType = "twoPin"
	} else if len(g.limitSwitchPins) == 0 {
		g.limitType = "encoderLengths"
		// encoder not supported currently.
	} else {
		np := len(g.limitSwitchPins)
		na := len(g.motorList)
		return nil, errors.Errorf("invalid gantry type: need 1, 2 or 0 pins per axis, have %v pins for %v axes", np, na)
	}

	g.axes = config.Attributes.BoolSlice("axes", true)

	g.rpm = config.Attributes.Float64Slice("rpm")

	err := g.init(ctx)
	if err != nil {
		return nil, err
	}

	return g, nil
}

// Multiaxis is a gantry type that includes lists of motor names, a list of motor objects, a limit board with enable pins,
// motor directionpins, limit positionpins and a descriptive name based on the type of limit used.
// It also includes motor initial speeds, and each axes' length as descriptors.
// AxesList is not doing much now.
type multiAxis struct {
	name      string
	motorList []string
	motors    []motor.Motor

	limitSwitchPins []string
	limitBoard      board.Board
	limitHigh       bool
	axes            []bool
	limitType       string
	pulleyR         []float64

	lengthMeters []float64
	rpm          []float64

	positionLimits []float64

	logger golog.Logger
}

// Initialization function differs based on gantry type, one limit switch and length,
// two limit switch pins and an encoder

func (g *multiAxis) init(ctx context.Context) error {
	// Mapping one limit switch motor0->limsw0, motor1 ->limsw1, motor 2 -> limsw2
	// Mapping two limit switch motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	for idx := range g.motorList {
		motorID := idx
		switch g.limitType {
		case "oneLimSwitchOneLength":
			limitIDs := []int{idx}
			err := g.homeOneLimSwitch(ctx, motorID, limitIDs)
			if err != nil {
				return err
			}
		case "twoLimSwitch":
			limitIDs := []int{2 * idx, 2*idx + 1}
			err := g.homeTwoLimSwitch(ctx, motorID, limitIDs)
			if err != nil {
				return err
			}
		case "encoder":
			err := g.homeEncoder(ctx, motorID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// home gets passed to init.
func (g *multiAxis) homeTwoLimSwitch(ctx context.Context, motorID int, limitID []int) error {
	ok, err := g.motors[motorID].PositionSupported(ctx)
	if err != nil {
		return err
	}

	if !ok {
		return errors.Errorf("gantry motor %v needs to support position", motorID)
	}

	// Finds and stores first position of this motor (backwards limit) for this axis.
	positionA, err := g.testLimit(ctx, motorID, limitID, true)
	if err != nil {
		return err
	}

	// Finds and stores first position of this motor (forwards limit) for this axis.
	positionB, err := g.testLimit(ctx, motorID, limitID, false)
	if err != nil {
		return err
	}

	g.logger.Debugf("positionA: %0.02f positionB: %0.02f", positionA, positionB)

	g.positionLimits = []float64{positionA, positionB}

	return nil
}

func (g *multiAxis) homeOneLimSwitch(ctx context.Context, motorID int, limitID []int) error {
	ok, err := g.motors[motorID].PositionSupported(ctx)
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("gantry motor needs to support position")
	}

	// One pin always and only should go backwards.
	positionA, err := g.testLimit(ctx, motorID, limitID, true)
	if err != nil {
		return err
	}

	radius := g.pulleyR[motorID]
	stepsPerLength := g.lengthMeters[motorID] / (radius * 2 * math.Pi)

	positionB := positionA + stepsPerLength

	g.positionLimits = []float64{positionA, positionB}

	return nil
}

// Not yet implemented.
func (g *multiAxis) homeEncoder(ctx context.Context, motorID int) error {
	return errors.New("encoder currently not supported")
}

func (g *multiAxis) testLimit(ctx context.Context, motorID int, limitID []int, zero bool) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motors[motorID].Stop(ctx)
	})

	// Gantry starts going backwards always. Limit switch must be placed in backwards direction from the motor.
	// Important for defining limit pins for direction.
	dir := float64(-1)
	offset := limitID[0]
	// We will at most have two limit switches per axis.
	if !zero {
		dir = float64(1)
		offset = limitID[1]
	}

	// Each motor goes for an bounded amount of time until it hits the limit switch.
	err := g.motors[motorID].GoFor(ctx, dir*g.rpm[motorID], 10000)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, offset)
		if err != nil {
			return 0, err
		}
		if hit {
			err = g.motors[motorID].Stop(ctx)
			if err != nil {
				return 0, err
			}
			break
		}

		// Needs to find limit switch within 15 seconds. Initial rpm and length is a consideration here that the user needs
		// to be aware of.
		elapsed := start.Sub(start)
		if elapsed > (time.Second * 15) {
			return 0, errors.New("gantry timed out testing limit")
		}

		if !utils.SelectContextOrWait(ctx, time.Millisecond*10) {
			return 0, ctx.Err()
		}
	}
	return g.motors[motorID].Position(ctx)
}

// This reads a limit pin from the list of limitPins and returns true if that limit pin is hit.
func (g *multiAxis) limitHit(ctx context.Context, offset int) (bool, error) {
	pin := g.limitSwitchPins[offset]
	high, err := g.limitBoard.GPIOGet(ctx, pin)

	return high == g.limitHigh, err
}

func (g *multiAxis) CurrentPosition(ctx context.Context) ([]float64, error) {
	posOut := make([]float64, len(g.motorList))
	for idx := range g.motorList {
		pos, err := g.motors[idx].Position(ctx)
		if err != nil {
			return nil, err
		}
		theRange := g.positionLimits[2*idx+1] - g.positionLimits[2*idx]

		targetPos := g.positionLimits[2*idx] + (pos * theRange)

		limit1, err := g.limitHit(ctx, 2*idx)
		if err != nil {
			return nil, err
		}
		limit2, err := g.limitHit(ctx, 2*idx+1)
		if err != nil {
			return nil, err
		}

		g.logger.Debugf("multiAxis axis %v CurrentPosition %.02f -> %.02f. limSwitch1: %t, limSwitch2: %t", idx, pos, targetPos, limit1, limit2)

		posOut[idx] = targetPos
	}

	return posOut, nil
}

func (g *multiAxis) Lengths(ctx context.Context) ([]float64, error) {
	lengthsout := []float64{}
	for idx := range g.motorList {
		lengthsout = append(lengthsout, g.lengthMeters[idx])
	}
	return lengthsout, nil
}

func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	if len(positions) != len(g.motors) {
		return errors.Errorf(" gantry MoveToPosition needs %v positions, got: %v", len(g.motorList), len(positions))
	}

	for idx := range g.motorList {
		if positions[idx] < 0 || positions[idx] > g.lengthMeters[idx] {
			return errors.Errorf("gantry position for axis %v is out of range, got %0.02f max is %0.02f", idx, positions[0], g.lengthMeters[idx])
		}
	}

	switch g.limitType {
	// Moving should be the same for single limit switch logic and two limit switch logic
	case "oneLimSwitch", "twoLimSwitch":
		for idx := range g.motorList {
			theRange := g.positionLimits[2*idx+1] - g.positionLimits[2*idx]

			// can change into x, y, z explicitly without a for loop if preferable.
			targetPos := positions[idx] / g.lengthMeters[idx]
			targetPos = g.positionLimits[2*idx] + (targetPos * theRange)

			g.logger.Debugf("gantry axis %v SetPosition %0.02f -> %0.02f", idx, positions[idx], targetPos)

			// Hits backwards limit switch, goes in forwards direction for two revolutions
			hit, err := g.limitHit(ctx, idx)
			if err != nil {
				return err
			}
			if hit {
				if targetPos < g.positionLimits[2*idx] {
					dir := float64(1)
					return g.motors[idx].GoFor(ctx, dir*g.rpm[idx], 2)
				}
				return g.motors[idx].Stop(ctx)
			}

			// Hits forward limit switch, goes in backwards direction for two revolutions
			hit, err = g.limitHit(ctx, 2*idx+1)
			if err != nil {
				return err
			}
			if hit {
				if targetPos > g.positionLimits[2*idx+1] {
					dir := float64(-1)
					return g.motors[idx].GoFor(ctx, dir*g.rpm[idx], 2)
				}
				return g.motors[idx].Stop(ctx)
			}

			err = g.motors[idx].GoTo(ctx, g.rpm[idx], targetPos)
			if err != nil {
				return err
			}
		}

	case "encoder":
		// TODO: implementation.
		return errors.New("no encoders supported")
	}
	return nil
}

<<<<<<< HEAD
//TODO incorporate frames into movement function above.
func (g *multiAxis) ModelFrame() *referenceframe.Model {
	m := referenceframe.NewModel()
=======
// TODO incorporate frames into movement function above.
func (g *multiAxis) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
>>>>>>> c51caed0 (addressed smaller requested changes)
	for idx := range g.motorList {
		f, err := referenceframe.NewTranslationalFrame(
			g.name,
			[]bool{true},
			[]referenceframe.Limit{{0, g.lengthMeters[idx]}},
		)
		if err != nil {
			panic(fmt.Errorf("error creating frame %v, should be impossible %w", idx, err))
		}
		m.OrdTransforms = append(m.OrdTransforms, f)
	}

	return m
}

// Will be used in motor movement function above.
func (g *multiAxis) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}

func (g *multiAxis) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}
