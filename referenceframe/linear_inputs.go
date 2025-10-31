package referenceframe

import (
	"fmt"
	"iter"
)

type FrameInputsMeta struct {
	frameName string

	// Having both Offset and DoF is merely a convenience. Only having one would suffice.
	offset int
	dof    int
}

// FrameSystemInputs is an alias for a mapping of frame names to slices of Inputs.
type LinearInputs struct {
	// Cache map[string][]Input
	meta   []FrameInputsMeta
	inputs []Input
}

func NewLinearInputs() *LinearInputs {
	return &LinearInputs{
		meta:   make([]FrameInputsMeta, 0, 8),
		inputs: make([]Input, 0, 8),
	}
}

func LinearInputsFromMetaAndData(meta []FrameInputsMeta, inputs []Input) (*LinearInputs, error) {
	totalDoF := 0
	for _, fMeta := range meta {
		totalDoF += fMeta.dof
	}
	if totalDoF != len(inputs) {
		return nil, fmt.Errorf("wrong number of inputs. expected: %v received: %v", totalDoF, len(inputs))
	}

	return &LinearInputs{
		meta:   meta,
		inputs: inputs,
	}, nil
}

func FrameInputMetaFromFramesystem(fs *FrameSystem) []FrameInputsMeta {
	ret := make([]FrameInputsMeta, 0, 8)
	offsetAcc := 0
	for name, frame := range fs.frames {
		numDof := len(frame.DoF())
		if numDof == 0 {
			continue
		}

		ret = append(ret, FrameInputsMeta{
			frameName: name,
			offset:    offsetAcc,
			dof:       numDof,
		})

		offsetAcc += numDof
	}

	return ret
}

func FrameInputMetaToSlice(meta []FrameInputsMeta, fs *LinearInputs) ([]float64, error) {
	ret := make([]float64, 0, 8)
	for _, fMeta := range meta {
		if fMeta.dof == 0 {
			continue
		}

		fInputs := fs.Get(fMeta.frameName)
		if len(fInputs) != fMeta.dof {
			return nil, fmt.Errorf("frame has wrong number of inputs. frame: %v expected %v received: %v",
				fMeta.frameName, fMeta.dof, len(fInputs))
		}

		ret = append(ret, fInputs...)
	}

	return ret, nil
}

func (li *LinearInputs) Len() int {
	return len(li.inputs)
}

func (li *LinearInputs) Put(frameName string, inputs []Input) {
	for _, meta := range li.meta {
		if meta.frameName != frameName {
			continue
		}

		if meta.dof != len(inputs) {
			// Don't reposition the underlying arrays. It's not expected for callers need to change
			// the number of inputs for a given frame.
			return
		}

		copy(li.inputs[meta.offset:], inputs)
		return
	}

	li.meta = append(li.meta, FrameInputsMeta{
		frameName: frameName,
		offset:    len(li.inputs),
		dof:       len(inputs),
	})
	li.inputs = append(li.inputs, inputs...)
}

func (li *LinearInputs) Get(frameName string) []Input {
	for _, meta := range li.meta {
		if meta.frameName == frameName {
			return li.inputs[meta.offset : meta.offset+meta.dof]
		}
	}

	return nil
}

func (li *LinearInputs) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, meta := range li.meta {
			if !yield(meta.frameName) {
				return
			}
		}
	}
}

func (li *LinearInputs) Items() iter.Seq2[string, []Input] {
	return func(yield func(string, []Input) bool) {
		for _, meta := range li.meta {
			if !yield(meta.frameName, li.inputs[meta.offset:meta.offset+meta.dof]) {
				return
			}
		}
	}
}

// GetFrameInputs returns the inputs corresponding to the given frame within the LinearInputs object.
func (li *LinearInputs) GetFrameInputs(frame Frame) ([]Input, error) {
	if len(frame.DoF()) == 0 {
		return nil, nil
	}

	ret := li.Get(frame.Name())
	if ret == nil {
		return nil, fmt.Errorf("no inputs for frame %s with dof: %d", frame.Name(), len(frame.DoF()))
	}

	return ret, nil
}

// ComputePoses computes the poses for each frame in a framesystem in frame of World, using the provided configuration.
func (li *LinearInputs) ComputePoses(fs *FrameSystem) (FrameSystemPoses, error) {
	// Compute poses from configuration using the FrameSystem
	// computedPoses := make(FrameSystemPoses)
	// for _, frameName := range fs.FrameNames() {
	//  	pif, err := fs.Transform(li, NewZeroPoseInFrame(frameName), World)
	//  	if err != nil {
	//  		return nil, err
	//  	}
	//  	computedPoses[frameName] = pif.(*PoseInFrame)
	// }

	// return computedPoses, nil

	return nil, nil
}

func FromOldLinearInputs(li map[string][]Input) *LinearInputs {
	ret := NewLinearInputs()
	for frameName, inputs := range li {
		ret.Put(frameName, inputs)
	}

	return ret
}
