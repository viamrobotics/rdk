package armplanning

import (
	"testing"

	"go.viam.com/rdk/logging"
)

// newChattyMotionPlanTestLogger returns a logger that suppresses most of the logging output. Motion
// planning is sensitive to timing (e.g: 1 second to generate IK solutions), hence large amounts of
// output that the invoking process is slow to read from can cause large pauses.
//
// Some of the chattier tests overproduce output because there are many goals. Either explicitly
// from the user request, or because `generateWaypoints` implicitly creates a lot. If they fail,
// we're committing to reproducing locally rather than deducting from logs.
//
// Which can be a reasonable strategy. Motion planning is collection of heuristics. Any specific
// test failing isn't because a single heuristic failed, but some set of them all coincidentally
// failed for some run.
func newChattyMotionPlanTestLogger(tb testing.TB) logging.Logger {
	logger, _, reg := logging.NewObservedTestLoggerWithRegistry(tb, tb.Name())
	reg.Update([]logging.LoggerPatternConfig{
		{
			Pattern: "*.mp*",
			Level:   "WARN",
		},
	}, logger)

	return logger
}
