// Package framesystem defines and implements the concept of a frame system.
package framesystem

import (
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/referenceframe"
)

// NewFrameSystemFromParts assembles a frame system from a collection of parts,
// usually acquired by calling Config on a frame system service.
func NewFrameSystemFromParts(name, prefix string, parts Parts, logger golog.Logger) (referenceframe.FrameSystem, error) {
	// ensure that at least one frame connects to world if the frame system is not empty
	if len(parts) != 0 {
		hasWorld := false
		for _, part := range parts {
			if part.FrameConfig.Parent() == referenceframe.World {
				hasWorld = true
				break
			}
		}
		if !hasWorld {
			return nil, errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
		}
	}
	// Topologically sort parts
	sortedParts, err := TopologicallySort(parts)
	if err != nil {
		return nil, err
	}
	if len(sortedParts) != len(parts) {
		return nil, errors.Errorf(
			"frame system has disconnected frames. connected frames: %v, all frames: %v",
			Names(sortedParts),
			Names(parts),
		)
	}
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for _, part := range sortedParts {
		// rename everything with prefixes
		part.FrameConfig.SetName(prefix + part.FrameConfig.Name())
		// prefixing for the world frame is only necessary in the case
		// of merging multiple frame systems together, so we leave that
		// reponsibility to the corresponding merge function
		if part.FrameConfig.Parent() != referenceframe.World {
			part.FrameConfig.SetParent(prefix + part.FrameConfig.Parent())
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := referenceframe.CreateFramesFromPart(part)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.Frame(part.FrameConfig.Parent()))
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

func frameNamesWithDof(sys referenceframe.FrameSystem) []string {
	names := sys.FrameNames()
	nameDoFs := make([]string, len(names))
	for i, f := range names {
		fr := sys.Frame(f)
		nameDoFs[i] = fmt.Sprintf("%s(%d)", fr.Name(), len(fr.DoF()))
	}
	return nameDoFs
}
