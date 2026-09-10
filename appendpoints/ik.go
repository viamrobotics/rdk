package appendpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/motionplan/armplanning/mpserver"
)

func readRequestFromJsonBytes(reader io.Reader) (*armplanning.PlanRequest, error) {
	decoder := json.NewDecoder(reader)

	// We first decode the file into a raw json structure. This is because we have best effort
	// support for reading different versions of request files. The current version of the
	// `PlanRequest` object may not map perfectly to some historical serialization.
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}

	// We've removed world state from the plan request object. Instead forcing callers to merge
	// world state transforms into the `FrameSystem` member itself. And world state obstacles are
	// now passed in directly.
	req := &armplanning.PlanRequest{}
	if _, hasWorldState := probe["world_state"]; hasWorldState {
		// Legacy format, parse as a `PlanRequestWithWorldState` and have that "upgrade" to a modern
		// `PlanRequest`.
		legacy := &armplanning.PlanRequestWithWorldState{}
		if err := json.Unmarshal(raw, legacy); err != nil {
			return nil, err
		}

		// Dan: We explicitly ignore world state transforms when "upgrading" from this legacy
		// request file. This was introduced because of a bug where world state transforms weren't
		// being used at all. So this preserves that behavior. But maybe it's better to do merging
		// as that was always the intent?
		var err error
		req, err = legacy.ToPlanRequestWorldStateTransformsIgnored()
		if err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(raw, req); err != nil {
			return nil, err
		}
	}

	// RSDK-14172: Plan requests can have a nil PlannerOptions. PlanMotion (currently) substitutes
	// in the `NewBasicPlannerOptions`.
	if req.PlannerOptions == nil {
		req.PlannerOptions = armplanning.NewBasicPlannerOptions()
	}

	return req, nil
}

type Solution struct {
	Cost               float64               `json:"cost"`
	Configuration      map[string][]float64  `json:"configuration"`
	ConfigurationValid bool                  `json:"configuration_valid"`
	ConfigurationError *string               `json:"error,omitempty"`
	CheckpathValid     *bool                 `json:"checkpath_valid,omitempty"`
	FirstError         *string               `json:"first_error,omitempty"`
	LastGoodInputs     *map[string][]float64 `json:"last_good_inputs,omitempty"`
}

type IKSeedResult struct {
	Seed      string     `json:"seed"`
	Solutions []Solution `json:"solutions"`
}

func IKHandler(resp http.ResponseWriter, req *http.Request) {
	ctx := context.Background()
	logger := logging.NewBlankLogger("ik-handler")

	defer req.Body.Close()
	planRequest, err := readRequestFromJsonBytes(req.Body)
	if err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf(
			"Error parsing armplanning request. Error: %v\n",
			err.Error())))
		return
	}

	var numSolutions int = 10
	numSolutionsStr := req.URL.Query().Get("numSolutions")
	if numSolutionsStr != "" {
		numSolutions, err = strconv.Atoi(numSolutionsStr)
		if err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte(fmt.Sprintf(
				"Error parsing `numSolutions`. Must be an integer. Error: %v\n",
				err.Error())))
			return
		}
	}

	results, err := mpserver.InspectIK(
		ctx, logger, planRequest,
		planRequest.StartState.Configuration(),
		planRequest.Goals[0].Poses(),
		numSolutions)
	if err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf(
			"Error generating IK solutions. Error: %v\n",
			err.Error())))
	}

	fmt.Printf("NumLabels: %v NumRows: %v FirstRowLen: %v\n",
		len(results.SeedLabels),
		len(results.SeedResults),
		len(results.SeedResults[0]))

	var jsonResp []*IKSeedResult
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

		jsonResp = append(jsonResp, ikSeedResult)
	}

	if err := json.NewEncoder(resp).Encode(jsonResp); err != nil {
		resp.WriteHeader(500)
		resp.Write([]byte(fmt.Sprintf(
			"Error JSON serializing ik solutions. Error: %v\n", err.Error())))
	}
}
