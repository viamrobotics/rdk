package multiaxisgantry

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/component/gantry"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterComponent(gantry.Subtype, "multiaxis", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMultiAxis(ctx, r, config, logger)
		}})
}

// NewMultiAxis creates a new-multi axis gantry
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
	}

	var ok bool

	// The multi axis gantry uses the motors to consturct the axes of the gantry
	// Logic considering # axes != to # of motors should be added if we need to use reference
	// frames in the future.
	g.motorList = config.Attributes.StringSlice("motors")
	if len(g.motorList) < 1 {
		return nil, errors.New("Need at least 1 axis.")
	} else if len(g.motorList) > 2 {
		return nil, errors.New("Up to 3 axes supported")
	}

	for idx := 0; idx < len(g.motorList); idx++ {
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
	if len(g.lengthMeters) <= len(g.motors) {
		return nil, errors.New("Each axis needs a non-zero length.")
	}

	for idx := 0; idx < len(g.motorList); idx++ { //fix for loop syntax
		if g.lengthMeters[idx] <= 0 {
			return nil, errors.New("All axes must have non-zero length")
		}
	}

	g.limitSwitchPins = config.Attributes.StringSlice("limitPins")
	// 0 for encoder, equals number of axes for one limit pin per axes (Cheap 3D printer style gantry)
	// and 2*number of axes for more sensorized gantries.
	if len(g.limitSwitchPins) == len(g.motorList) {
		g.limitType = "onePinOneLength"
	} else if len(g.limitSwitchPins) == 2*len(g.motorList) {
		g.limitType = "twoPin"
	} else if len(g.limitSwitchPins) == 0 {
		g.limitType = "encoderLengths"
		// encoder not supported currently, need to think about how to incorprate pin and different types of encoders
		// in core a bit more.
	} else {
		return nil, errors.Errorf("Need one of: 1 limitPin per axis, 2 limitPin per axis or zero for encoders. We have %v motors, %v limitPins.", len(g.motorList), len(g.limitSwitchPins))
	}

	//g.axesList = config.Attributes.BoolSlice("axes", true)
	// Added this because I saw reference frames exist as lists of bools.
	// Not sure if needed in the future is not tested right now.

	g.rpm = config.Attributes.Float64Slice("rpm")

	err := g.init(ctx)
	if err != nil {
		return nil, err
	}

	return g, nil
}

// Multiaxis is a gantry type that includes lists of motor names, a list of motor objects, a limit board with enable pins,
// motor directionpins, limit positionpins and a descriptive name based on the type of limit used.
// It also includes motor initial speeds, and each axes' length as descriptots.
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

	lengthMeters []float64
	rpm          []float64

	positionLimits []float64

	logger golog.Logger
}

// Initialization function differs based on gantry type, one limit switch and length,
// two limit switch pins and an encoder

func (g *multiAxis) init(ctx context.Context) error {
	// count is used to ensure the mapping of motor0->limSw0,limSw1; motor1->limSw2,limSw3; motor2->limSw4,limSw5
	count := 0
	//TODO: Print indexes of motor and limit switch to make sure it works
	for idx := 0; idx < len(g.motors); idx++ {
		motorID := idx
		switch g.limitType {
		case "oneLimSwitchOneLength":
			limitIDs := []int{idx}
			g.homeOneLimSwitch(ctx, motorID, limitIDs)
		case "twoLimSwitch":
			limitIDs := []int{idx + count, idx + count + 1}
			g.homeTwoLimSwitch(ctx, motorID, limitIDs)
			count++
		case "encoder":
			g.homeEncoder(ctx, motorID)
		}
	}
	return nil
}

// home gets passed to init
func (g *multiAxis) homeTwoLimSwitch(ctx context.Context, motorID int, limitID []int) error {
	ok, err := g.motors[motorID].PositionSupported(ctx)
	if err != nil {
		return err
	}

	if !ok {
		return errors.Errorf("gantry motor %v needs to support position.", motorID)
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

	// TODO: Check if index makes sense.
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

	radius := .02
	// totally random number set to current gantry pulley radius in ,meters
	// length, do we have a wheel component anywhere? or an easy way of seeing how much distance is travelled per tick?
	// Alternatively we need an absolute measurement of the pulley radius/wheel raius/leadscrew radius.
	// This section probably needs a lot of rethinkiing to generalize
	stepsPerLength := g.lengthMeters[motorID] / radius

	// singlestep := g.motors[motorID].TicksPerRotation / math.Pi
	positionB := positionA + stepsPerLength //If I understand GPIO stepper right

	g.positionLimits = []float64{positionA, positionB}

	return nil
}

// Not supported.
func (g *multiAxis) homeEncoder(ctx context.Context, motorID int) error {
	return fmt.Errorf("encoder currently not supported")
}

func (g *multiAxis) testLimit(ctx context.Context, motorID int, limitID []int, zero bool) (float64, error) {
	defer utils.UncheckedErrorFunc(func() error {
		return g.motors[motorID].Off(ctx)
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

	// Each motor goes for an inordinate amount of time until it hits the limit switch.
	err := g.motors[motorID].GoFor(ctx, dir*g.rpm[motorID], 10000)
	if err != nil {
		return 0, err
	}

	//
	start := time.Now()
	for {
		hit, err := g.limitHit(ctx, offset)
		if err != nil {
			return 0, err
		}
		if hit {
			err = g.motors[motorID].Off(ctx)
			if err != nil {
				return 0, err
			}
			break
		}

		// needs to find limit switch within 15 seconds. Initial rpm and length is a consideration here that the user needs
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

// This reads a limit pin from the list of limitPins and returns true if that limit pin // is hit.
func (g *multiAxis) limitHit(ctx context.Context, offset int) (bool, error) {
	pin := g.limitSwitchPins[offset]
	high, err := g.limitBoard.GPIOGet(ctx, pin)

	return high == g.limitHigh, err
}

func (g *multiAxis) CurrentPosition(ctx context.Context) ([]float64, error) {
	count := 0
	posOut := []float64{}
	for idx := 0; idx < len(g.motors); idx++ {
		pos, err := g.motors[idx].Position(ctx)
		if err != nil {
			return nil, err
		}
		theRange := g.positionLimits[idx+1+count] - g.positionLimits[idx+count]

		// can change into x, y, z explicitly without a for loop if preferable.
		targetPos := g.positionLimits[idx+count] + (pos * theRange)

		limit1, _ := g.limitHit(ctx, idx+count)
		limit2, _ := g.limitHit(ctx, idx+count+1)

		g.logger.Debugf("oneAxis CurrentPosition %.02f -> %.02f. limSwitch1: %t, limSwitch2: %t", pos, targetPos, limit1, limit2)

		return append(posOut, targetPos), nil
	}

	return posOut, nil
}

func (g *multiAxis) Lengths(ctx context.Context) ([]float64, error) {
	lengthsout := []float64{}
	for idx := 0; idx < len(g.motors); idx++ {
		return append(lengthsout, g.lengthMeters[idx]), nil
	}
	return lengthsout, nil
}

func (g *multiAxis) MoveToPosition(ctx context.Context, positions []float64) error {
	if len(positions) != len(g.motors) {
		return fmt.Errorf(" gantry MoveToPosition needs %v positions, got: %v", len(g.motorList), len(positions))
	}

	for idx := 0; idx < len(g.motorList); idx++ {
		if positions[idx] < 0 || positions[idx] > g.lengthMeters[idx] {
			return fmt.Errorf("gantry position for axis %v is out of range, got %0.02f max is %0.02f", idx, positions[0], g.lengthMeters[idx])
		}
	}

	switch g.limitType {
	// Moving should be the same for single limit switch logic and two limit switch logic

	case "oneLimSwitch", "twoLimSwitch":
		count := 0
		for idx := 0; idx < len(g.motors); idx++ {
			theRange := g.positionLimits[idx+1+count] - g.positionLimits[idx+count]

			// can change into x, y, z explicitly without a for loop if preferable.
			targetPos := positions[idx] / g.lengthMeters[idx]
			targetPos = g.positionLimits[idx+count] + (targetPos * theRange)

			g.logger.Debugf("gantry axis %v SetPosition %0.02f -> %0.02f", idx, positions[idx], targetPos)

			// Hits backwards limit switch, goes in forwards direction for two revolutions
			hit, err := g.limitHit(ctx, idx)
			if err != nil {
				return err
			}
			if hit {
				if targetPos < g.positionLimits[idx+count] {
					dir := float64(1)
					return g.motors[idx].GoFor(ctx, dir*g.rpm[idx], 2)
				} else {
					return g.motors[idx].Off(ctx)
				}
			}

			// Hits forward limit switch, goes in backwards direction for two revolutions
			hit, err = g.limitHit(ctx, idx+count+1)
			if err != nil {
				return err
			}
			if hit {
				if targetPos > g.positionLimits[idx+1+count] {
					dir := float64(-1)
					return g.motors[idx].GoFor(ctx, dir*g.rpm[idx], 2)
				} else {
					return g.motors[idx].Off(ctx)
				}
			}
			count++

			return g.motors[idx].GoTo(ctx, g.rpm[idx], targetPos)
		}

	case "encoder":
		//unsupported - have to think about what encoders will do or support here,
		// or if it is useful to have our own calibration of an encoder gantry anyway.
		return fmt.Errorf("no encoders supported")
	}
	return nil
}

// This might be completely wrong. Not entirely sure how to use frames yet, my understanding is that
// this is appending the translational frames of each axis to a model of the entire gantry.
// Probably wrong.
func (g *multiAxis) ModelFrame() *referenceframe.Model {
	m := referenceframe.NewModel()
	for idx := 0; idx < len(g.motorList); idx++ {
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

// This will eventually lead to an overhaul of the above code, but since we're not currently usign reference frames to move, I'll leave it here.
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
