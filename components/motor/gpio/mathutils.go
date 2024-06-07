package gpio

import (
	"fmt"
	"math"
	"time"

	"go.viam.com/rdk/components/motor"
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

// If revolutions is 0, the returned wait duration will be 0 representing that
// the motor should run indefinitely.
func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	if revolutions == 0 {
		powerPct := rpm / maxRPM
		return powerPct, 0
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

// goForMath calculates goalPos, goalRPM, and direction based on the given GoFor rpm and revolutions, and the current position.
func encodedGoForMath(rpm, revolutions, currentPos, ticksPerRotation float64) (float64, float64, float64) {
	direction := sign(rpm * revolutions)
	if revolutions == 0 {
		direction = sign(rpm)
	}

	goalPos := (math.Abs(revolutions) * ticksPerRotation * direction) + currentPos
	goalRPM := math.Abs(rpm) * direction

	if revolutions == 0 {
		goalPos = math.Inf(int(direction))
	}

	return goalPos, goalRPM, direction
}

func checkSpeed(rpm, max float64) (string, error) {
	switch speed := math.Abs(rpm); {
	case speed == 0:
		return "motor speed requested is 0 rev_per_min", motor.NewZeroRPMError()
	case speed > 0 && speed < 0.1:
		return "motor speed is nearly 0 rev_per_min", nil
	case max > 0 && speed > max-0.1:
		return fmt.Sprintf("motor speed is nearly the max rev_per_min (%f)", max), nil
	default:
		return "", nil
	}
}
