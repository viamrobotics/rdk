package referenceframe

import (
	"fmt"
	"strings"

	"github.com/golang/geo/r3"
)

// ChainFrame is one frame on the path from a model's base to one of its frames, with where its
// inputs sit in the model's input vector. Composing the frames of a chain in order is Transform.
type ChainFrame struct {
	// Frame is a static, revolute or prismatic frame in a model read from a file, never one holding
	// another model. A model built from frames in code may also carry a frame with several inputs,
	// such as a pose frame, which comes back as it is.
	Frame Frame
	// Offset is the index of the frame's first input in the model's input vector, 0 for a frame
	// with no inputs and -1 for one that follows another.
	Offset int
	// Mimic describes the input of a frame that follows another, and is nil otherwise.
	Mimic *MimicInput
}

// MimicInput derives a frame's input as ValueMultiplier times the input at Source plus ValueOffset.
type MimicInput struct {
	Source          int
	ValueMultiplier float64
	ValueOffset     float64
}

// RevoluteFrame is a frame that turns about one axis and takes one input, in radians.
type RevoluteFrame interface {
	Frame
	// RotationAxis is the unit axis the frame turns about, in its parent's frame.
	RotationAxis() r3.Vector
}

// PrismaticFrame is a frame that slides along one axis and takes one input, in millimeters.
type PrismaticFrame interface {
	Frame
	// TranslationAxis is the unit axis the frame slides along, in its parent's frame.
	TranslationAxis() r3.Vector
}

// Chain returns the frames from the model's base to outputFrame in order. An empty outputFrame
// means the frame Transform reports, and the world frame or a model with no frames gives an empty
// chain. A frame holding another model, as a model built over a frame system of parts has, is
// replaced by that model's chain with its offsets shifted, so the frames come back under the
// names the part's model gives them, and a frame renamed with NewNamedFrame comes back as the
// frame underneath so that the axis interfaces assert on it. The error names a frame the model
// does not have.
func (m *SimpleModel) Chain(outputFrame string) ([]ChainFrame, error) {
	if outputFrame == "" {
		outputFrame = m.primaryOutputFrame
	}
	if outputFrame == World {
		return nil, nil
	}
	if m.internalFS == nil || m.internalFS.lookupFrame(outputFrame) == nil {
		return nil, NewFrameMissingError(outputFrame)
	}
	w := &chainWalk{offsets: map[*SimpleModel]map[string]int{}}
	return w.chain(m, outputFrame)
}

// chainWalk memoizes where each model's frames start in its input vector.
type chainWalk struct {
	offsets map[*SimpleModel]map[string]int
}

func (w *chainWalk) chain(m *SimpleModel, outputFrame string) ([]ChainFrame, error) {
	var names []string
	for name := outputFrame; name != World; {
		names = append(names, name)
		parent, ok := m.internalFS.parents[name]
		if !ok {
			return nil, NewFrameMissingError(name)
		}
		name = parent
	}
	var chain []ChainFrame
	for i := len(names) - 1; i >= 0; i-- {
		entries, err := w.entries(m, names[i])
		if err != nil {
			return nil, err
		}
		chain = append(chain, entries...)
	}
	return chain, nil
}

// entries is what one frame of the model contributes to a chain, a whole shifted chain for a
// frame that holds another model.
func (w *chainWalk) entries(m *SimpleModel, name string) ([]ChainFrame, error) {
	if owner, internal := m.internalFS.internalToComponent[name]; internal {
		// climb past parts nested in parts, which the frame system flattens under both names
		for {
			above, ok := m.internalFS.internalToComponent[owner]
			if !ok {
				break
			}
			owner = above
		}
		nested := asFlattenableModel(m.internalFS.lookupFrame(owner))
		if nested == nil {
			return nil, fmt.Errorf("frame %q of model %q belongs to %q, which holds no model", name, m.name, owner)
		}
		base, err := w.offset(m, owner, nested)
		if err != nil {
			return nil, err
		}
		entries, err := w.entries(nested, strings.TrimPrefix(name, owner+":"))
		if err != nil {
			return nil, err
		}
		return shiftChain(entries, base), nil
	}
	frame := m.internalFS.lookupFrame(name)
	if nested := asFlattenableModel(frame); nested != nil {
		base, err := w.offset(m, name, nested)
		if err != nil {
			return nil, err
		}
		entries, err := w.chain(nested, nested.primaryOutputFrame)
		if err != nil {
			return nil, err
		}
		return shiftChain(entries, base), nil
	}
	entry := ChainFrame{Frame: unwrapNamed(frame)}
	if len(frame.DoF()) == 0 {
		return []ChainFrame{entry}, nil
	}
	if mm := m.mimicMappings[name]; mm != nil {
		entry.Offset = -1
		entry.Mimic = &MimicInput{Source: mm.sourceInputIdx, ValueMultiplier: mm.valueMultiplier, ValueOffset: mm.valueOffset}
		return []ChainFrame{entry}, nil
	}
	offset, err := w.offset(m, name, frame)
	if err != nil {
		return nil, err
	}
	entry.Offset = offset
	return []ChainFrame{entry}, nil
}

func (w *chainWalk) offset(m *SimpleModel, name string, frame Frame) (int, error) {
	if len(frame.DoF()) == 0 {
		return 0, nil
	}
	index, ok := w.offsets[m]
	if !ok {
		index = map[string]int{}
		if m.inputSchema != nil {
			for _, meta := range m.inputSchema.metas {
				index[meta.frameName] = meta.offset
			}
		}
		w.offsets[m] = index
	}
	offset, ok := index[name]
	if !ok {
		return 0, fmt.Errorf("frame %q of model %q takes inputs the model does not carry", name, m.name)
	}
	return offset, nil
}

// shiftChain moves a freshly built nested chain's offsets in place to where the nested model's
// inputs sit.
func shiftChain(chain []ChainFrame, base int) []ChainFrame {
	for i := range chain {
		if len(chain[i].Frame.DoF()) == 0 {
			continue
		}
		if chain[i].Mimic != nil {
			chain[i].Mimic.Source += base
			continue
		}
		chain[i].Offset += base
	}
	return chain
}
