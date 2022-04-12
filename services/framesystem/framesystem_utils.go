// Package framesystem defines and implements the concept of a frame system.
package framesystem

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// RobotFrameSystem returns the frame system of the robot.
func RobotFrameSystem(
	ctx context.Context,
	r robot.Robot,
	additionalTransforms []*commonpb.Transform,
) (referenceframe.FrameSystem, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::RobotFrameSystem")
	defer span.End()
	frameService, err := FromRobot(r)
	if err != nil {
		return nil, err
	}
	// create the frame system
	allParts, err := frameService.Config(ctx, additionalTransforms)
	if err != nil {
		return nil, err
	}
	fs, err := NewFrameSystemFromParts(LocalFrameSystemName, "", allParts, r.Logger())
	if err != nil {
		return nil, err
	}
	return fs, nil
}

// NewFrameSystemFromParts assembles a frame system from a collection of parts,
// usually acquired by calling Config on a frame system service.
func NewFrameSystemFromParts(
	name, prefix string, parts Parts,
	logger golog.Logger,
) (referenceframe.FrameSystem, error) {
	// ensure that at least one frame connects to world if the frame system is not empty
	if len(parts) != 0 {
		hasWorld := false
		for _, part := range parts {
			if part.FrameConfig.Parent == referenceframe.World {
				hasWorld = true
				break
			}
		}
		if !hasWorld {
			return nil, errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
		}
	}
	// Topologically sort parts
	sortedParts, err := TopologicallySortParts(parts)
	if err != nil {
		return nil, err
	}
	if len(sortedParts) != len(parts) {
		return nil, errors.Errorf(
			"frame system has disconnected frames. connected frames: %v, all frames: %v",
			partNames(sortedParts),
			partNames(parts),
		)
	}
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for _, part := range sortedParts {
		// rename everything with prefixes
		part.Name = prefix + part.Name
		// prefixing for the world frame is only necessary in the case
		// of merging multiple frame systems together, so we leave that
		// reponsibility to the corresponding merge function
		if part.FrameConfig.Parent != referenceframe.World {
			part.FrameConfig.Parent = prefix + part.FrameConfig.Parent
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, logger)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.GetFrame(part.FrameConfig.Parent))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(fs))
	return fs, nil
}

// combineParts combines the local, remote, and offset parts into one slice.
// Renaming of the remote parts does not happen in this function.
func combineParts(
	localParts Parts,
	offsetParts map[string]*config.FrameSystemPart,
	remoteParts map[string]Parts,
) Parts {
	allParts := Parts{}
	allParts = append(allParts, localParts...)
	allParts = append(allParts, partMapToPartSlice(offsetParts)...)
	for _, part := range remoteParts {
		allParts = append(allParts, part...)
	}
	return allParts
}

// robotFrameSystemConfig returns the frame system parts of the robot through the frame system service.
func robotFrameSystemConfig(ctx context.Context, r robot.Robot) (Parts, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem::RobotFrameSystemConfig")
	defer span.End()
	fsSrv, err := FromRobot(r)
	if err != nil {
		return nil, err
	}
	parts, err := fsSrv.Config(ctx, []*commonpb.Transform{})
	if err != nil {
		return nil, err
	}
	return parts, nil
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func extractModelFrameJSON(r robot.Robot, name resource.Name) (referenceframe.Model, error) {
	part, err := r.ResourceByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "no resource found with name %q when extracting model frame json", name)
	}
	if framer, ok := utils.UnwrapProxy(part).(referenceframe.ModelFramer); ok {
		return framer.ModelFrame(), nil
	}
	return nil, referenceframe.ErrNoModelInformation
}

// getRemoteRobotConfig gets the parameters for the Remote.
func getRemoteRobotConfig(remoteName string, conf *config.Config) (*config.Remote, error) {
	for _, rConf := range conf.Remotes {
		if rConf.Name == remoteName {
			return &rConf, nil
		}
	}
	return nil, fmt.Errorf("cannot find Remote config with name %q", remoteName)
}

func frameNamesWithDof(sys referenceframe.FrameSystem) []string {
	names := sys.FrameNames()
	nameDoFs := make([]string, len(names))
	for i, f := range names {
		fr := sys.GetFrame(f)
		nameDoFs[i] = fmt.Sprintf("%s(%d)", fr.Name(), len(fr.DoF()))
	}
	return nameDoFs
}
