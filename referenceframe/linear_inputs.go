package referenceframe

import (
	"fmt"
	"iter"
	"sort"
)

// LinearInputsMeta
type LinearInputsSchema struct {
	metas []linearInputMeta
}

func InputSchemaFromFrameSystem(fs *FrameSystem) (*LinearInputsSchema, error) {
	metas := []linearInputMeta{}

	offset := 0
	frameNames := fs.FrameNames()
	sort.Strings(frameNames)
	for _, fName := range frameNames {
		frame := fs.Frame(fName)
		if frame == nil {
			return nil, fmt.Errorf("frame %s was returned in list of frame names, but was not found in frame system", fName)
		}

		metas = append(metas, linearInputMeta{
			frameName: fName,
			offset:    offset,
			dof:       len(frame.DoF()),
		})
		offset += len(frame.DoF())
	}

	return &LinearInputsSchema{
		metas: metas,
	}, nil
}

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

func (lis *LinearInputsSchema) FloatsToInputsStack(inps []float64) (LinearInputs, error) {
	totDoF := 0
	for idx := range lis.metas {
		totDoF += lis.metas[idx].dof
	}
	if totDoF != len(inps) {
		return LinearInputs{}, fmt.Errorf("wrong number of inputs. Expected: %v Received: %v", totDoF, len(inps))
	}

	return LinearInputs{
		schema: lis,
		inputs: inps,
	}, nil
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

func LinearInputsSchemaFromFrameSystem(fs *FrameSystem) *LinearInputsSchema {
	metas := make([]linearInputMeta, 0, 8)
	offsetAcc := 0
	for name, frame := range fs.frames {
		numDof := len(frame.DoF())
		if numDof == 0 {
			continue
		}

		metas = append(metas, linearInputMeta{
			frameName: name,
			offset:    offsetAcc,
			dof:       numDof,
		})

		offsetAcc += numDof
	}

	return &LinearInputsSchema{metas}
}

func (li *LinearInputs) Len() int {
	return len(li.inputs)
}

func (li *LinearInputs) GetLinearizedInputs() []Input {
	return li.inputs
}

func (li *LinearInputs) GetSchema() *LinearInputsSchema {
	return li.schema
}

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

func (li *LinearInputs) Get(frameName string) []Input {
	for _, meta := range li.schema.metas {
		if meta.frameName == frameName {
			return li.inputs[meta.offset : meta.offset+meta.dof]
		}
	}

	return nil
}

func (li *LinearInputs) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, meta := range li.schema.metas {
			if !yield(meta.frameName) {
				return
			}
		}
	}
}

func (li *LinearInputs) Items() iter.Seq2[string, []Input] {
	return func(yield func(string, []Input) bool) {
		for _, meta := range li.schema.metas {
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
	return li.ToFrameSystemInputs().ComputePoses(fs)
}

func (li *LinearInputs) ToFrameSystemInputs() FrameSystemInputs {
	ret := make(FrameSystemInputs)
	for frameName, inputs := range li.Items() {
		ret[frameName] = inputs
	}

	return ret
}
