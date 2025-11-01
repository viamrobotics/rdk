package referenceframe

import (
	"fmt"
	"iter"
)

// LinearInputsSchema describes the order of frame names and their degrees of freedom in a set of
// Inputs. This is important for communicating consistently with IK/nlopt.
type LinearInputsSchema struct {
	metas []linearInputMeta
}

// FloatsToInputs applies the given schema to a new set of linearized floats. This returns an error
// if the wrong number of floats are provided.
func (lis *LinearInputsSchema) FloatsToInputs(inps []float64) (*LinearInputs, error) {
	totDoF := 0
	for idx := range lis.metas {
		totDoF += lis.metas[idx].dof
	}
	if totDoF != len(inps) {
		return nil, fmt.Errorf("wrong number of inputs. Expected: %v Received: %v", totDoF, len(inps))
	}

	return &LinearInputs{
		schema: lis,
		inputs: inps,
	}, nil
}

// FrameNamesInOrder returns the frame names in schema order.
func (lis *LinearInputsSchema) FrameNamesInOrder() []string {
	ret := make([]string, len(lis.metas))
	for idx, meta := range lis.metas {
		ret[idx] = meta.frameName
	}

	return ret
}

type linearInputMeta struct {
	frameName string

	// Having both Offset and DoF is merely a convenience to keep information local. Only having one
	// would suffice. But then we'd need to compare adjacent `linearInputMeta` in the schema array
	// of `metas`.
	offset int
	dof    int
}

// LinearInputs is a memory optimized representation of FrameSystemInputs. The type is expected to
// only be used by direct consumers of the frame system library.
type LinearInputs struct {
	// Cache map[string][]Input ?
	schema *LinearInputsSchema
	inputs []Input
}

// NewLinearInputs initializes a LinearInputs.
func NewLinearInputs() *LinearInputs {
	return &LinearInputs{
		schema: &LinearInputsSchema{},
		inputs: make([]Input, 0, 8),
	}
}

// Len returns how many frames (included 0-DoF frames) are in the LinearInputs.
func (li *LinearInputs) Len() int {
	return len(li.schema.metas)
}

// GetLinearizedInputs returns the flat array of floats. Used for communicating with IK/nlopt.
func (li *LinearInputs) GetLinearizedInputs() []Input {
	if li == nil {
		return []Input{}
	}

	return li.inputs
}

// GetSchema returns the underlying `LinearInputsSchema` associated with this LinearInputs. When
// using this LinearInputs to create inputs for IK/nlopt, this returned `LinearInputsSchema` is how
// to turn the IK/nlopt output back into frames inputs.
func (li *LinearInputs) GetSchema() *LinearInputsSchema {
	return li.schema
}

// Put adds a new frameName -> inputs mapping. Put will overwrite an existing mapping when the frame
// name matches and the input value is of the same DoF as the prior mapping. Put will silently
// ignore the request if an overwrite has a different DoF than the original. That is considered
// programmer error.
func (li *LinearInputs) Put(frameName string, inputs []Input) {
	for _, meta := range li.schema.metas {
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

	li.schema.metas = append(li.schema.metas, linearInputMeta{
		frameName: frameName,
		offset:    len(li.inputs),
		dof:       len(inputs),
	})
	li.inputs = append(li.inputs, inputs...)
}

// Get returns the inputs associated with a frame name.
func (li *LinearInputs) Get(frameName string) []Input {
	if li == nil {
		return []Input{}
	}

	for _, meta := range li.schema.metas {
		if meta.frameName == frameName {
			return li.inputs[meta.offset : meta.offset+meta.dof]
		}
	}

	return nil
}

// Keys returns an iterator over the keys. This is analogous to ranging over the keys of a map.
func (li *LinearInputs) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, meta := range li.schema.metas {
			if !yield(meta.frameName) {
				return
			}
		}
	}
}

// Items returns an iterator over the key-value pairs in this `LinearInputs`. This is analogous to
// the key/value ranging over a map.
func (li *LinearInputs) Items() iter.Seq2[string, []Input] {
	return func(yield func(string, []Input) bool) {
		for _, meta := range li.schema.metas {
			if !yield(meta.frameName, li.inputs[meta.offset:meta.offset+meta.dof]) {
				return
			}
		}
	}
}

// GetFrameInputs returns the inputs corresponding to the given frame within the LinearInputs
// object. This method returns an error if the frame has non-zero DoF and the frame name does not
// exist.
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

// ComputePoses computes the poses for each frame in a framesystem in frame of World, using the
// provided configuration.
func (li *LinearInputs) ComputePoses(fs *FrameSystem) (FrameSystemPoses, error) {
	// This method is not expected to be called in hot paths. Reuse the
	// `FrameSystemPoses.ComputePoses` method.
	return li.ToFrameSystemInputs().ComputePoses(fs)
}

// ToFrameSystemInputs creates a `FrameSystemInputs` with the same keys and values. This is a
// convenience method for interfacing with higher level public APIs that don't need to be as
// efficient.
func (li *LinearInputs) ToFrameSystemInputs() FrameSystemInputs {
	ret := make(FrameSystemInputs)
	for frameName, inputs := range li.Items() {
		ret[frameName] = inputs
	}

	return ret
}
