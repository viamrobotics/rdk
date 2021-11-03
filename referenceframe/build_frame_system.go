package referenceframe

import (
	"context"
	"fmt"
	"sort"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
)

func BuildFrameSystem(ctx context.Context, name string, frameNames map[string]bool, children map[string][]Frame, logger golog.Logger) (FrameSystem, error) {
	// use a stack to populate the frame system
	stack := make([]string, 0)
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children[World]; !ok {
		return nil, nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, World)
	// begin adding frames to the frame system
	fs := NewEmptySimpleFrameSystem(name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, nil, errors.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			err := fs.AddFrame(frame, fs.GetFrame(parent))
			if err != nil {
				return nil, nil, err
			}
		}
	}
	// ensure that there are no disconnected frames
	if len(visited) != len(frameNames) {
		return nil, nil, errors.Errorf("the frame system is not fully connected, expected %d frames but frame system has %d. Expected frames are: %v. Actual frames are: %v", len(frameNames), len(visited), mapKeys(frameNames), mapKeys(visited))
	}
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(ctx, fs))
	return fs, nil
}

func frameNamesWithDof(ctx context.Context, sys FrameSystem) []string {
	names := sys.FrameNames()
	nameDoFs := make([]string, len(names))
	for i, f := range names {
		fr := sys.GetFrame(f)
		nameDoFs[i] = fmt.Sprintf("%s(%d)", fr.Name(), len(fr.DoF(ctx)))
	}
	return nameDoFs
}

func mapKeys(fullmap map[string]bool) []string {
	keys := make([]string, len(fullmap))
	i := 0
	for k := range fullmap {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
