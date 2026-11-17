package motion

import (
	"context"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/motionplan"
)

// DoCommand keys understood by the builtin motion service.
const (
	DoPlan                = "plan"
	DoPlanPartialWaypoint = "plan_partialwp"
	DoExecute             = "execute"
	DoExecuteCheckStart   = "executeCheckStart"
)

// PlanResult is the decoded response of a DoPlan DoCommand.
type PlanResult struct {
	Trajectory motionplan.Trajectory
	// PartialWaypoint is non-nil when the planner returned a partial plan, and holds the
	// number of goals the trajectory reaches.
	PartialWaypoint *int
}

// Plan asks svc to generate a trajectory for req without executing it, using the DoPlan DoCommand.
func Plan(ctx context.Context, svc Service, req MoveReq) (PlanResult, error) {
	cmd, err := EncodePlanCommand(svc.Name().ShortName(), req)
	if err != nil {
		return PlanResult{}, err
	}
	resp, err := svc.DoCommand(ctx, cmd)
	if err != nil {
		return PlanResult{}, err
	}
	return DecodePlanResponse(resp)
}

// EncodePlanCommand builds the DoCommand map for a DoPlan request.
func EncodePlanCommand(name string, req MoveReq) (map[string]interface{}, error) {
	reqPB, err := req.ToProto(name)
	if err != nil {
		return nil, err
	}
	bytes, err := protojson.Marshal(reqPB)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{DoPlan: string(bytes)}, nil
}

// DecodePlanResponse decodes a DoPlan DoCommand response, whether it was produced in-process
// or deserialized from the wire.
func DecodePlanResponse(resp map[string]interface{}) (PlanResult, error) {
	raw, ok := resp[DoPlan]
	if !ok || raw == nil {
		return PlanResult{}, errors.Errorf("DoCommand response missing %q key; motion service may not support planning via DoCommand", DoPlan)
	}
	var result PlanResult
	if err := mapstructure.Decode(raw, &result.Trajectory); err != nil {
		return PlanResult{}, errors.Wrap(err, "decoding trajectory")
	}
	if rawWP, ok := resp[DoPlanPartialWaypoint]; ok {
		var wp int
		if err := mapstructure.Decode(rawWP, &wp); err != nil {
			return PlanResult{}, errors.Wrap(err, "decoding partial waypoint")
		}
		result.PartialWaypoint = &wp
	}
	return result, nil
}
