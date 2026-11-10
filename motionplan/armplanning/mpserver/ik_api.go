package mpserver

import (
	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
)

// Solution contains information regarding an nlopt IK solution.
type Solution struct {
	Cost               float64               `json:"cost"`
	Configuration      map[string][]float64  `json:"configuration"`
	ConfigurationValid bool                  `json:"configuration_valid"`
	ConfigurationError *string               `json:"error,omitempty"`
	CheckpathValid     *bool                 `json:"checkpath_valid,omitempty"`
	FirstError         *string               `json:"first_error,omitempty"`
	LastGoodInputs     *map[string][]float64 `json:"last_good_inputs,omitempty"`
}

// IKSeedResult ties together a "seed" or scenario for IK solutions and a sampling of solutions
// generated from that seed.
type IKSeedResult struct {
	Seed      string     `json:"seed"`
	Solutions []Solution `json:"solutions"`
}

// IKHandler generates a table of solutions for an input plan request.
func IKHandler(planRequest *armplanning.PlanRequest, numSolutions int) ([]*IKSeedResult, error) {
	ctx := context.Background()
	logger := logging.NewBlankLogger("ik-handler")

	results, err := InspectIK(
		ctx, logger, planRequest,
		planRequest.StartState.Configuration(),
		planRequest.Goals[0].Poses(),
		numSolutions,
	)
	if err != nil {
		return nil, err
	}

	var ret []*IKSeedResult
	for seedIdx, seedName := range results.SeedLabels {
		ikSeedResult := &IKSeedResult{
			Seed: seedName,
		}

		for _, cell := range results.SeedResults[seedIdx] {
			ikSeedResult.Solutions = append(ikSeedResult.Solutions, Solution{
				Cost:               cell.Cost,
				ConfigurationValid: cell.Valid,
			})
			solution := &ikSeedResult.Solutions[len(ikSeedResult.Solutions)-1]

			if cell.Inputs == nil {
				// IK failed. Typically when a seed puts tighter limits on solutions that it can
				// generate.
				continue
			}

			solution.Configuration = cell.Inputs.ToFrameSystemInputs()
			if !cell.Valid {
				errStr := cell.StateError.Error()
				solution.ConfigurationError = &errStr
				continue
			}

			if cell.CheckPathOK {
				cpValid := true
				solution.CheckpathValid = &cpValid
				continue
			}

			cpValid := false
			solution.CheckpathValid = &cpValid
			firstErrorStr := cell.CheckPathError.Error()
			solution.FirstError = &firstErrorStr
			if lastGood := cell.CheckPathFeedback.LastGoodInputs; lastGood != nil {
				var config map[string][]float64 = lastGood.ToFrameSystemInputs()
				solution.LastGoodInputs = &config
			}
		}

		ret = append(ret, ikSeedResult)
	}

	return ret, nil
}
