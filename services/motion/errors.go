package motion

import "github.com/pkg/errors"

// ErrGoalWithinPlanDeviation is an error describing when planning fails becuase there is nothing to be done.
var ErrGoalWithinPlanDeviation = errors.New("no need to move, already within planDeviationMM")
