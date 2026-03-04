package armplanning

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"go.viam.com/rdk/logging"
)

//go:embed data/wine-adjust.json
var wineAdjustJSON []byte

// this is how fast the machine is at motion planning
// 1 is m5 macbook
// 2 is slower
// .5 is faster
var speedMultiplier = 2.0

func init() {
	logger := logging.NewLogger("startup-profile")
	logger.SetLevel(logging.WARN)

	avgTime, err := checkStartupPerf(logger)
	if err != nil {
		logger.Warnf("checkStartupPerf failed: %v", err)
	} else {
		speedMultiplier = float64(avgTime.Milliseconds()) / 45
		logger.Infof("avgTime: %v speedMultiplier: %v", avgTime, speedMultiplier)
	}
}

func checkStartupPerf(logger logging.Logger) (time.Duration, error) {
	totalTime := time.Duration(0)
	countRuns := 0

	req, err := readRequestFromBytes(wineAdjustJSON)
	if err != nil {
		return totalTime, err
	}

	for i := range 4 {
		start := time.Now()
		_, _, err = PlanMotion(context.Background(), logger, req)
		if err != nil {
			return totalTime, err
		}
		if i > 0 {
			totalTime += time.Since(start)
			countRuns++
		}
	}

	return totalTime / time.Duration(countRuns), nil
}

func readRequestFromBytes(data []byte) (*PlanRequest, error) {
	req := &PlanRequest{}
	err := json.NewDecoder(bytes.NewReader(data)).Decode(req)
	if err != nil {
		return nil, err
	}
	return req, nil
}
