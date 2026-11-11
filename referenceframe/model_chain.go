package referenceframe

import (
	"fmt"
	"strings"

	"github.com/golang/geo/r3"
)

// ChainFrame is one frame on the path from a model's base to one of its frames, with where the
// frame's inputs sit in the model's input vector. Chain returns them in order from the base, so
// composing each frame's Transform onto the one before gives the pose of the last, which is
// how the model computes Transform for its output frame.
type ChainFrame struct {
	// Frame is never a frame holding another model, since Chain expands those. In a model read
	// from a file it is a static frame, a revolute frame, which implements RevoluteFrame, or a
	// prismatic frame, which implements PrismaticFrame. A model built from frames in code may
	// also carry a frame with several inputs, such as a pose frame, which comes back as it is,
	// so a consumer that cannot drive one has to refuse it by its DoF.
	Frame Frame
	// Offset is the index of the frame's first input in the model's input vector. It is 0 for a
	// frame with no inputs, which has nothing to read there, and -1 for a frame that follows
	// another, which Mimic describes.
	Offset int
	// Mimic is set for a frame whose input is derived from another input of the model rather
	// than read from the input vector, and nil otherwise.
	Mimic *MimicInput
}

// MimicInput says how a frame that follows another gets its input, which is ValueMultiplier
// times the input at Source in the model's input vector, plus ValueOffset.
type MimicInput struct {
	Source          int
	ValueMultiplier float64
	ValueOffset     float64
}

// RevoluteFrame is a frame that turns about one axis and takes one input, in radians.
type RevoluteFrame interface {
	Frame
	// RotationAxis is the unit axis the frame turns about, in the frame of its parent, with
	// which the frame coincides at an input of zero.
	RotationAxis() r3.Vector
}

// PrismaticFrame is a frame that slides along one axis and takes one input, in millimeters.
type PrismaticFrame interface {
	Frame
	// TranslationAxis is the unit axis the frame slides along, in the frame of its parent, with
	// which the frame coincides at an input of zero.
	TranslationAxis() r3.Vector
}

// Chain returns the frames from the model's base to outputFrame in order, with where each one's
// inputs sit in the model's input vector. An empty outputFrame means the output frame whose pose
// Transform reports. A model with no frames, which a resource without kinematics reports, has an
// identity Transform and an empty chain, as does the world frame itself.
//
// A frame holding another model, which a model built over a frame system of parts has one of
// per part, is replaced by that model's chain with its offsets shifted to where the part's
// inputs sit, and a frame the frame system flattened out of a part is read as the matching
// frame of the part's model, so climbing through a part's frames reads the same as holding the
// part whole. Those frames come back under the names the part's model gives them, not the
// part:frame names the frame system shows. A frame renamed with NewNamedFrame comes back as the
// frame underneath, so that the axis interfaces assert on it, under its original name.
//
// The error names a frame the model does not have.
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

// chainWalk keeps, for each model a chain passes through, where its frames start in its input
// vector, since a chain into a part asks for the part's place once per frame of the part.
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

// entries is the part of a chain one frame of the model contributes. A frame holding another
// model contributes that model's whole chain, and a frame flattened out of a part contributes
// the matching frame of the part's model, both shifted to where the part's inputs sit in this
// model's input vector.
func (w *chainWalk) entries(m *SimpleModel, name string) ([]ChainFrame, error) {
	if owner, internal := m.internalFS.internalToComponent[name]; internal {
		// a part's model may hold a part of its own, whose frames the frame system flattens under
		// both names, so we climb to the frame of this model that holds them all and let that
		// model place the rest
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

// offset is where a frame's inputs start in a model's input vector. A frame with no inputs has
// no place there and reads as 0.
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

// shiftChain moves a nested chain's offsets to where the nested model's inputs sit in the
// enclosing model's input vector. The chain was built for this call and nothing else holds it,
// so it is changed in place. A frame with no inputs keeps its 0.
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
