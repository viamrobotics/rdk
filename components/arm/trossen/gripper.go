// Package trossen implements arms from Trossen Robotics.
package trossen

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/utils"
)

// DoCommand handles incoming gripper requests
func (a *Arm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
	switch name {
	case "grab":
		grabbed, err := a.Grab(ctx)
		return map[string]interface{}{"grabbed": grabbed}, err 
	case "open":
		return nil, a.Open(ctx)
	default:
		return nil, errors.Errorf("no such command: %s", name)
	}
}

// Open function to be used by generic DoCommand based gripper
func (a *Arm) Open(ctx context.Context) error {
	ctx, done := a.opMgr.New(ctx)
	defer done()
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	err := a.Joints["Gripper"][0].SetGoalPWM(150)
	if err != nil {
		return err
	}

	// We don't want to over-open
	atPos := false
	for !atPos {
		var pos int
		pos, err = a.Joints["Gripper"][0].PresentPosition()
		if err != nil {
			return err
		}
		if pos < 2800 {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		} else {
			atPos = true
		}
	}
	err = a.Joints["Gripper"][0].SetGoalPWM(0)
	return err
}

// Grab function to be used by generic DoCommand based gripper
func (a *Arm) Grab(ctx context.Context) (bool, error) {
	_, done := a.opMgr.New(ctx)
	defer done()
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	err := a.Joints["Gripper"][0].SetGoalPWM(-350)
	if err != nil {
		return false, err
	}
	err = servo.WaitForMovementVar(a.Joints["Gripper"][0])
	if err != nil {
		return false, err
	}
	pos, err := a.Joints["Gripper"][0].PresentPosition()
	if err != nil {
		return false, err
	}
	didGrab := true

	// If servo position is less than 1500, it's closed and we grabbed nothing
	if pos < 1500 {
		didGrab = false
	}
	return didGrab, nil
}
