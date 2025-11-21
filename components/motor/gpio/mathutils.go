package gpio

import (
	"fmt"
	"math"
	"time"

	"go.viam.com/rdk/components/encoder"
)

func fixPowerPct(powerPct, max float64) float64 {
	powerPct = math.Min(powerPct, max)
	powerPct = math.Max(powerPct, -1*max)
	return powerPct
}

func sign(x float64) float64 { // A quick helper function
	if x == 0 {
		return 0
	}
	if math.Signbit(x) {
		return -1.0
	}
	return 1.0
}

func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	dir := sign(rpm * revolutions)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

// goForMath calculates goalPos, goalRPM, and direction based on the given GoFor rpm and revolutions, and the current position.
func encodedGoForMath(rpm, revolutions, currentPos, ticksPerRotation float64) (float64, float64, float64) {
	direction := sign(rpm * revolutions)

	goalPos := (math.Abs(revolutions) * ticksPerRotation * direction) + currentPos
	goalRPM := math.Abs(rpm) * direction

	return goalPos, goalRPM, direction
}

// checkEncPosType checks that the position type of an encoder is in ticks.
func checkEncPosType(posType encoder.PositionType) error {
	if posType != encoder.PositionTypeTicks {
		return fmt.Errorf("expected %v got %v", encoder.PositionTypeTicks.String(), posType.String())
	}
	return nil
}
