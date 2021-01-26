package support

import (
	"fmt"

	"github.com/edaniels/golog"
)

type FakeBase struct {
	PosX, PosY     int
	BoundX, BoundY int
}

func (fb *FakeBase) MoveStraight(distanceMM int, speed int, block bool) error {
	return nil
}

func (fb *FakeBase) Spin(degrees int, power int, block bool) error {
	return nil
}

func (fb *FakeBase) Stop() error {
	return nil
}

func (fb *FakeBase) String() string {
	return fmt.Sprintf("pos: (%d, %d)", fb.PosX, fb.PosY)
}

type MoveDir string

const (
	MoveDirUp    = MoveDir("up")
	MoveDirLeft  = MoveDir("left")
	MoveDirDown  = MoveDir("down")
	MoveDirRight = MoveDir("right")
)

func (fb *FakeBase) Move(dir MoveDir, amount int) error {
	errMsg := fmt.Errorf("cannot move %q; stuck", dir)
	switch dir {
	case MoveDirUp:
		if fb.PosY-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("up", "amount", amount)
		fb.PosY -= amount
	case MoveDirLeft:
		if fb.PosX-amount < 0 {
			return errMsg
		}
		golog.Global.Debugw("left", "amount", amount)
		fb.PosX -= amount
	case MoveDirDown:
		if fb.PosY+amount >= fb.BoundY {
			return errMsg
		}
		golog.Global.Debugw("down", "amount", amount)
		fb.PosY += amount
	case MoveDirRight:
		if fb.PosX+amount >= fb.BoundX {
			return errMsg
		}
		golog.Global.Debugw("right", "amount", amount)
		fb.PosX += amount
	default:
		return fmt.Errorf("unknown direction %q", dir)
	}
	return nil
}

func (fb *FakeBase) Close() {

}
