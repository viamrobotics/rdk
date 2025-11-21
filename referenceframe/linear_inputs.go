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

// GetSchema returns the underlying `LinearInputsSchema` associated with this LinearInputs. When
// using this LinearInputs to create inputs for IK/nlopt, this returned `LinearInputsSchema` is how
// to turn the IK/nlopt output back into frames inputs. The `FrameSystem` is an additional input
// such that we can ensure the proper joint limits are assigned.
func (li *LinearInputs) GetSchema(fs *FrameSystem) (*LinearInputsSchema, error) {
	for idx, meta := range li.schema.metas {
		frame := fs.Frame(meta.frameName)
		if frame == nil {
			return nil, NewFrameMissingError(meta.frameName)
		}

		limits := frame.DoF()
		if len(limits) != meta.dof {
			return nil, fmt.Errorf("incorrect dof for frame %s %w",
				meta.frameName, NewIncorrectDoFError(len(limits), meta.dof))
		}

		li.schema.metas[idx].frame = frame
	}

	// We also walk the framesystem and add any missing frames to the LinearInputs (with 0 value
	// inputs) and schema.
	//
	// Dan: I'm unsure if this is the right behavior. Perhaps we should error if we've been given
	// _some_ before `GetSchema` was called, and now we're learning about more? Certainly in the
	// motion planning case, we expect to have all of the current inputs before going into
	// nlopt. Lest we risk changing inputs for frames that we don't have a known start position
	// for. We cannot properly do collision detection for those frames.
	for _, frameName := range fs.FrameNames() {
		if li.Get(frameName) != nil {
			continue
		}

		frame := fs.Frame(frameName)
		offset := len(li.inputs)

		newMeta := linearInputMeta{
			frameName: frameName,
			offset:    offset,
			dof:       len(frame.DoF()),
			frame:     frame,
		}
		li.schema.metas = append(li.schema.metas, newMeta)
		li.inputs = append(li.inputs, make([]Input, newMeta.dof)...)
	}

	return li.schema, nil
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

// GetLimits returns the limits associated with the frames in the linearized order expected by
// nlopt.
func (lis *LinearInputsSchema) GetLimits() []Limit {
	ret := make([]Limit, 0, len(lis.metas))
	for _, meta := range lis.metas {
		ret = append(ret, meta.frame.DoF()...)
	}

	return ret
}

// Jog returns a value that's one "jog" away from its current value. Where a jog is a fraction
// (`percentJog`) of the total range of the input.
func (lis *LinearInputsSchema) Jog(linearizedInputIdx int, val, percentJog float64) float64 {
	// This function is expected to be called infrequently, hence this linear scan ought to be
	// performant enough. This saves us from (the cognitive load) of having to keep around a
	// separate structure.
	metas := lis.metas
	for linearizedInputIdx >= len(metas[0].frame.DoF()) {
		// While the linearizedInputIdx does not belong to the "current frame", pull off the current
		// frame and subtract our target.
		linearizedInputIdx -= len(metas[0].frame.DoF())
		metas = metas[1:]
	}

	//nolint: revive
	_, max, r := metas[0].frame.DoF()[linearizedInputIdx].GoodLimits()
	x := r * percentJog

	val += x
	if val > max {
		// If we've gone too far, wrap around. This assumes the input is a rotational joint.
		val -= (2 * x)
	}

	return val
}

type linearInputMeta struct {
	frameName string

	// Having both Offset and DoF is merely a convenience to keep information local. Only having one
	// would suffice. But then we'd need to compare adjacent `linearInputMeta` in the schema array
	// of `metas`.
	offset int
	dof    int

	// frame is initialized when calling `LinearInputs.GetSchema`. Frames are not necessary when
	// doing transformations. But they are necessary when working with IK/nlopt.
	frame Frame
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

// CopyWithZeros makes a new copy with everything zero
func (li *LinearInputs) CopyWithZeros() *LinearInputs {
	return &LinearInputs{
		schema: li.schema,
		inputs: make([]Input, len(li.inputs)),
	}
}
