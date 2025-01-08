// Package experimental contains functions which extend the motion/builtin resource with additional functionality
// the code contained within it comes with no stability guarantees and is likely to change often
package experimental

import (
	"context"
	"errors"

	"github.com/go-viper/mapstructure/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin"
)

// DoPlan is a helper function to wrap DoPlan (a utility inside builtin/DoCommand) with types that are easier to work with.
func DoPlan(ctx context.Context, ms motion.Service, req motion.MoveReq) (motionplan.Trajectory, error) {
	proto, err := req.ToProto(ms.Name().Name)
	if err != nil {
		return nil, err
	}
	bytes, err := protojson.Marshal(proto)
	if err != nil {
		return nil, err
	}
	resp, err := ms.DoCommand(ctx, map[string]interface{}{builtin.DoPlan: string(bytes)})
	if err != nil {
		return nil, err
	}
	respMap, ok := resp[builtin.DoPlan]
	if !ok {
		return nil, errors.New("could not find Trajectory in DoCommand response")
	}
	var trajectory motionplan.Trajectory
	err = mapstructure.Decode(respMap, &trajectory)
	return trajectory, err
}

// DoExecute is a helper function to wrap DoExecute (a utility inside builtin/DoCommand) with types that are easier to work with.
func DoExecute(ctx context.Context, ms motion.Service, traj motionplan.Trajectory) error {
	_, err := ms.DoCommand(ctx, map[string]interface{}{builtin.DoExecute: traj})
	return err
}
