package motion

import (
	"errors"
	"maps"

	"github.com/go-viper/mapstructure/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/motionplan"
)

// DoCommand keys understood by the builtin motion service.
const (
	// DoPlan plans a motion without executing it. The request value is a MoveReq (or, for
	// non-Go callers, a protojson-encoded pb.MoveRequest string). The response value is a
	// motionplan.Trajectory, both in-process and through the Go gRPC client:
	//
	//	resp, err := svc.DoCommand(ctx, map[string]interface{}{motion.DoPlan: req})
	//	traj := resp[motion.DoPlan].(motionplan.Trajectory)
	DoPlan = "plan"
	// DoPlanPartialWaypoint is set in a DoPlan response, as an int, when the planner returned a
	// partial plan. Its value is the number of goals the trajectory reaches.
	DoPlanPartialWaypoint = "plan_partialwp"
	DoExecute             = "execute"
	DoExecuteCheckStart   = "executeCheckStart"
)

// encodePlanCommand returns cmd with a MoveReq under DoPlan replaced by the protojson string the
// builtin service expects over the wire. cmd is returned unchanged if it holds no MoveReq.
func encodePlanCommand(name string, cmd map[string]interface{}) (map[string]interface{}, error) {
	var req MoveReq
	switch v := cmd[DoPlan].(type) {
	case MoveReq:
		req = v
	case *MoveReq:
		if v == nil {
			return nil, errors.New("nil MoveReq for DoPlan")
		}
		req = *v
	default:
		return cmd, nil
	}
	reqPB, err := req.ToProto(name)
	if err != nil {
		return nil, err
	}
	bytes, err := protojson.Marshal(reqPB)
	if err != nil {
		return nil, err
	}
	out := maps.Clone(cmd)
	out[DoPlan] = string(bytes)
	return out, nil
}

// decodePlanResponse converts the wire-shaped DoPlan response values in resp back into a
// motionplan.Trajectory and an int, in place. Values that don't decode are left as they are, so
// motion services with their own "plan" command are unaffected.
func decodePlanResponse(resp map[string]interface{}) {
	if raw, ok := resp[DoPlan]; ok && raw != nil {
		var traj motionplan.Trajectory
		if err := mapstructure.Decode(raw, &traj); err == nil {
			resp[DoPlan] = traj
		}
	}
	if raw, ok := resp[DoPlanPartialWaypoint]; ok && raw != nil {
		var wp int
		if err := mapstructure.Decode(raw, &wp); err == nil {
			resp[DoPlanPartialWaypoint] = wp
		}
	}
}
