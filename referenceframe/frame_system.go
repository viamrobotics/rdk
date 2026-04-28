package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/protoutils"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/logging"
	spatial "go.viam.com/rdk/spatialmath"
)

// World is the string "world", but made into an exported constant.
const World = "world"

// defaultPointDensity ensures we use the default value specified within the spatialmath package.
const defaultPointDensity = 0.

// FrameSystemPoses is an alias for a mapping of frame names to PoseInFrame.
type FrameSystemPoses map[string]*PoseInFrame

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// FrameConfig gives the frame's location relative to parent,
// and ModelFrame is an optional ModelJSON that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	FrameConfig *LinkInFrame
	ModelFrame  Model
}

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem struct {
	name           string
	world          Frame // separate from the map of frames so it can be detached easily
	frames         map[string]Frame
	parents        map[string]string
	cachedBFSNames []string

	// flattenedModels maps component name → original SimpleModel for models that were
	// flattened into individual frames. Used by smart seeding and other code that needs
	// the combined multi-DoF model rather than individual joint frames.
	flattenedModels map[string]*SimpleModel

	// componentSchemas maps component name → namespaced LinearInputsSchema for flattened models.
	// The schema frame names use the namespaced convention (e.g., "arm1:joint1").
	// Used to convert between flat component inputs and per-frame LinearInputs.
	componentSchemas map[string]*LinearInputsSchema

	// mimicFrames maps namespaced frame name → mimic info for flattened mimic joints.
	// composeTransforms uses this to derive inputs from the source frame.
	mimicFrames map[string]*mimicInfo
}

// mimicInfo describes a mimic joint relationship for a flattened frame.
type mimicInfo struct {
	sourceFrameName string  // namespaced source frame (e.g., "arm1:left_joint")
	multiplier      float64 // value multiplier
	offset          float64 // value offset
}

// NewEmptyFrameSystem creates a graph of Frames that have.
func NewEmptyFrameSystem(name string) *FrameSystem {
	worldFrame := NewZeroStaticFrame(World)
	return &FrameSystem{
		name:             name,
		world:            worldFrame,
		frames:           map[string]Frame{},
		parents:          map[string]string{},
		flattenedModels:  map[string]*SimpleModel{},
		componentSchemas: map[string]*LinearInputsSchema{},
		mimicFrames:      map[string]*mimicInfo{},
	}
}

// resolveFrameInputs is a fallback for when linearInputs.Get(frameName) returns nil.
// It checks if frameName belongs to a flattened model and, if so, extracts the right
// slice from a component-name-keyed entry in the LinearInputs.
func (sfs *FrameSystem) resolveFrameInputs(li *LinearInputs, frameName string) []Input {
	for componentName, schema := range sfs.componentSchemas {
		componentInputs := li.Get(componentName)
		if componentInputs == nil {
			continue
		}
		for _, meta := range schema.metas {
			if meta.frameName == frameName && meta.dof > 0 {
				return componentInputs[meta.offset : meta.offset+meta.dof]
			}
		}
	}
	return nil
}

// NewFrameSystem assembles a frame system from a set of parts and additional transforms.
func NewFrameSystem(name string, parts []*FrameSystemPart, additionalTransforms []*LinkInFrame) (*FrameSystem, error) {
	allParts := make([]*FrameSystemPart, 0, len(parts)+len(additionalTransforms))
	allParts = append(allParts, parts...)
	for _, tf := range additionalTransforms {
		transformPart, err := LinkInFrameToFrameSystemPart(tf)
		if err != nil {
			return nil, err
		}
		allParts = append(allParts, transformPart)
	}

	if len(allParts) != 0 {
		hasWorld := false
		for _, part := range allParts {
			if part.FrameConfig.Parent() == World {
				hasWorld = true
				break
			}
		}
		if !hasWorld {
			return nil, ErrNoWorldConnection
		}
	}

	// Topologically sort parts. After sorting, unlinked parts may reference frames
	// that will only exist after model flattening (e.g., "arm1:joint1"). Those will
	// be processed in a second pass after flattening.
	sortedParts, unlinkedParts := TopologicallySortParts(allParts)

	fs := NewEmptyFrameSystem(name)
	for _, part := range sortedParts {
		if err := addPartToFS(fs, part); err != nil {
			return nil, err
		}
	}

	// Second pass: try to add unlinked parts whose parents now exist in the FS
	// (e.g., frames parented to flattened internal frames like "arm1:base_link").
	if err := addUnlinkedParts(fs, unlinkedParts); err != nil {
		return nil, err
	}

	return fs, nil
}

// addPartToFS creates the model and static offset frames for a part, adds them to the
// frame system, and flattens multi-DoF SimpleModels for intermediate frame parenting.
func addPartToFS(fs *FrameSystem, part *FrameSystemPart) error {
	modelFrame, staticOffsetFrame, err := createFramesFromPart(part)
	if err != nil {
		return err
	}
	if err = fs.AddFrame(staticOffsetFrame, fs.Frame(part.FrameConfig.Parent())); err != nil {
		return err
	}
	if err = fs.AddFrame(modelFrame, staticOffsetFrame); err != nil {
		return err
	}

	// Additionally flatten multi-DoF SimpleModels for intermediate frame parenting
	if sm, ok := part.ModelFrame.(*SimpleModel); ok && len(sm.DoF()) > 0 {
		if err = flattenModelIntoFS(fs, sm, part.FrameConfig.Name(), staticOffsetFrame); err != nil {
			return err
		}
	}
	return nil
}

// addUnlinkedParts retries adding parts whose parents weren't available during the first
// pass (e.g., they reference flattened internal frames). Keeps retrying until no more
// progress is made.
func addUnlinkedParts(fs *FrameSystem, unlinked []*FrameSystemPart) error {
	for len(unlinked) > 0 {
		var stillUnlinked []*FrameSystemPart
		for _, part := range unlinked {
			if fs.Frame(part.FrameConfig.Parent()) != nil {
				if err := addPartToFS(fs, part); err != nil {
					return err
				}
			} else {
				stillUnlinked = append(stillUnlinked, part)
			}
		}
		if len(stillUnlinked) == len(unlinked) {
			// No progress — remaining parts can't be linked.
			strs := make([]string, len(stillUnlinked))
			for idx, part := range stillUnlinked {
				strs[idx] = part.FrameConfig.Name()
			}
			return fmt.Errorf("Cannot construct frame system. Some parts are not linked to the world frame. Parts: %v",
				strs)
		}
		unlinked = stillUnlinked
	}
	return nil
}

// World returns the root of the frame system, which is always named "world".
func (sfs *FrameSystem) World() Frame {
	return sfs.world
}

// Parent returns the parent Frame of the given Frame. It will return nil if the given frame is World.
// Parent returns the parent of the given frame. When the raw parent is an internal
// frame of a flattened model, the component's SimpleModel is returned instead so that
// public callers see component-level structure only.
func (sfs *FrameSystem) Parent(frame Frame) (Frame, error) {
	if !sfs.frameExists(frame.Name()) {
		return nil, NewFrameMissingError(frame.Name())
	}
	parentName, exists := sfs.parents[frame.Name()]
	if !exists {
		return nil, NewParentFrameNilError(frame.Name())
	}

	// If the raw parent is an internal flattened frame, return the component's
	// SimpleModel instead so callers see component-level names.
	if componentName := sfs.componentForInternalFrame(parentName); componentName != "" {
		if sm := sfs.Frame(componentName); sm != nil {
			return sm, nil
		}
	}

	return sfs.Frame(parentName), nil
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (sfs *FrameSystem) frameExists(name string) bool {
	_, ok := sfs.frames[name]
	return ok || name == World
}

// RemoveFrame will delete the given frame and all descendents from the frame system if it exists.
func (sfs *FrameSystem) RemoveFrame(frame Frame) {
	sfs.removeFrameRecursive(frame)
	sfs.cachedBFSNames = bfsFrameNames(sfs)
}

func (sfs *FrameSystem) removeFrameRecursive(frame Frame) {
	delete(sfs.frames, frame.Name())
	delete(sfs.parents, frame.Name())

	// Remove all descendents
	for childName, parentName := range sfs.parents {
		if parentName == frame.Name() {
			sfs.removeFrameRecursive(sfs.Frame(childName))
		}
	}
}

// Frame returns the Frame which has the provided name. It returns nil if the frame is not found in the FraneSystem.
func (sfs *FrameSystem) Frame(name string) Frame {
	if !sfs.frameExists(name) {
		return nil
	}
	if name == World {
		return sfs.world
	}
	return sfs.frames[name]
}

// TracebackFrame traces the parentage of the given frame up to the world, and returns the full list of frames in between.
// The list will include both the query frame and the world referenceframe, and is ordered from query to world.
//
// When the raw parent is an internal frame of a flattened model, the entire internal
// chain is skipped and the component's SimpleModel is inserted instead. This ensures
// callers see component-level frames rather than internal per-joint frames.
func (sfs *FrameSystem) TracebackFrame(query Frame) ([]Frame, error) {
	if !sfs.frameExists(query.Name()) {
		return nil, NewFrameMissingError(query.Name())
	}
	if query == sfs.world {
		return []Frame{query}, nil
	}
	parentName, exists := sfs.parents[query.Name()]
	if !exists {
		return nil, NewParentFrameNilError(query.Name())
	}

	// If the raw parent is an internal flattened frame, skip the entire internal
	// chain and jump to the component's SimpleModel.
	var nextParent Frame
	if componentName := sfs.componentForInternalFrame(parentName); componentName != "" {
		nextParent = sfs.Frame(componentName)
	} else {
		nextParent = sfs.Frame(parentName)
	}

	parents, err := sfs.TracebackFrame(nextParent)
	if err != nil {
		return nil, err
	}
	return append([]Frame{query}, parents...), nil
}

// componentForInternalFrame returns the component name that owns the given internal
// frame, or "" if the frame is not part of any flattened model.
func (sfs *FrameSystem) componentForInternalFrame(name string) string {
	for componentName, schema := range sfs.componentSchemas {
		for _, meta := range schema.metas {
			if meta.frameName == name {
				return componentName
			}
		}
	}
	return ""
}

// FrameNames returns the list of frame names registered in the frame system.
// Internal frames from flattened models (namespaced with ":") are hidden from this list.
// Use cachedBFSNames directly for internal operations that need all frames.
func (sfs *FrameSystem) FrameNames() []string {
	if len(sfs.flattenedModels) == 0 {
		return sfs.cachedBFSNames
	}
	internal := sfs.internalFrameNameSet()
	result := make([]string, 0, len(sfs.cachedBFSNames)-len(internal))
	for _, name := range sfs.cachedBFSNames {
		if !internal[name] {
			result = append(result, name)
		}
	}
	return result
}

// internalFrameNameSet returns the set of namespaced frame names belonging to flattened models.
func (sfs *FrameSystem) internalFrameNameSet() map[string]bool {
	result := make(map[string]bool)
	for _, schema := range sfs.componentSchemas {
		for _, name := range schema.FrameNamesInOrder() {
			result[name] = true
		}
	}
	return result
}

// AddFrame sets an already defined Frame into the system.
func (sfs *FrameSystem) AddFrame(frame, parent Frame) error {
	// check to see if parent is in system
	if parent == nil {
		return NewParentFrameNilError(frame.Name())
	}
	if !sfs.frameExists(parent.Name()) {
		return NewFrameMissingError(parent.Name())
	}

	// check if frame with that name is already in system
	if sfs.frameExists(frame.Name()) {
		return NewFrameAlreadyExistsError(frame.Name())
	}

	// add to frame system
	sfs.frames[frame.Name()] = frame
	sfs.parents[frame.Name()] = parent.Name()
	sfs.cachedBFSNames = bfsFrameNames(sfs)
	return nil
}

// Transform takes in a Transformable object and destination frame, and returns the pose from the first to the second. Positions
// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
func (sfs *FrameSystem) Transform(inputs *LinearInputs, object Transformable, dst string) (Transformable, error) {
	src := object.Parent()
	if src == dst {
		return object, nil
	}
	if !sfs.frameExists(src) {
		return nil, NewFrameMissingError(src)
	}
	srcFrame := sfs.Frame(src)
	if !sfs.frameExists(dst) {
		return nil, NewFrameMissingError(dst)
	}

	var tfParentDQ spatial.DualQuaternion
	var err error
	if _, ok := object.(*GeometriesInFrame); ok && src != World {
		// We don't want to apply the final transformation when that is taken care of by the geometries
		// This has to do with the way we decided to tie geometries to frames for ease of defining them in the model_json file
		// A frame is assigned a pose and a geometry and the two are not coupled together. This way you do can define everything relative
		// to the parent frame. So geometries are tied to the frame they are assigned to but we do not want to actually transform them
		// along the final transformation.
		parentName, exists := sfs.parents[srcFrame.Name()]
		if !exists {
			return nil, NewParentFrameNilError(srcFrame.Name())
		}
		tfParentDQ, err = sfs.transformFromParent(inputs, sfs.Frame(parentName), sfs.Frame(dst))
	} else {
		tfParentDQ, err = sfs.transformFromParent(inputs, srcFrame, sfs.Frame(dst))
	}
	if err != nil {
		return nil, err
	}
	return object.Transform(&PoseInFrame{dst, &tfParentDQ, src, nil}), nil
}

// TransformToDQ is like `Transform` except it outputs a `DualQuaternion` that can be converted into
// a `Pose`. The advantage of being more manual is to avoid memory allocations when unnecessary. As
// only a pointer to a `DualQuaternion` satisfies the `Pose` interface.
//
// This also avoids an allocation by accepting a frame name as the input rather than a
// `Transformable`. Saving the caller from making an allocation if the resulting pose is the only
// desired output.
func (sfs *FrameSystem) TransformToDQ(inputs *LinearInputs, frame, parent string) (
	spatial.DualQuaternion, error,
) {
	if !sfs.frameExists(frame) {
		return spatial.DualQuaternion{}, NewFrameMissingError(frame)
	}

	if !sfs.frameExists(parent) {
		return spatial.DualQuaternion{}, NewFrameMissingError(parent)
	}

	tfParent, err := sfs.transformFromParent(inputs, sfs.Frame(frame), sfs.Frame(parent))
	if err != nil {
		return spatial.DualQuaternion{}, err
	}

	ret := tfParent.Transformation(dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	})
	return spatial.DualQuaternion{Number: ret}, nil
}

// Name returns the name of the simpleFrameSystem.
func (sfs *FrameSystem) Name() string {
	return sfs.name
}

// MergeFrameSystem will combine two frame systems together, placing the world of systemToMerge at the "attachTo" frame in sfs.
// The frame where systemToMerge will be attached to must already exist within sfs, so should be added before Merge happens.
// Merging is necessary when including remote robots, dynamically building systems of robots, or mutating a robot after it
// has already been initialized. For example, two independent rovers, each with their own frame system, need to now know where
// they are in relation to each other and need to have their frame systems combined.
func (sfs *FrameSystem) MergeFrameSystem(systemToMerge *FrameSystem, attachTo Frame) error {
	attachFrame := sfs.Frame(attachTo.Name())
	if attachFrame == nil {
		return NewFrameMissingError(attachTo.Name())
	}

	// make a map where the parent frame name is the key and the slice of children frames is the value
	// Use cachedBFSNames to include internal flattened frames that are hidden from FrameNames().
	childrenMap := map[string][]Frame{}
	for _, name := range systemToMerge.cachedBFSNames {
		child := systemToMerge.Frame(name)
		if child == nil {
			continue
		}
		// Use raw parents map to preserve internal frame structure (Parent() masks internal frames).
		rawParentName, exists := systemToMerge.parents[name]
		if !exists {
			continue
		}
		parent := systemToMerge.Frame(rawParentName)
		if parent == nil {
			continue
		}
		childrenMap[parent.Name()] = append(childrenMap[parent.Name()], child)
	}

	// add every frame from systemToMerge to the base frame system.
	queue := []Frame{systemToMerge.World()}
	for len(queue) != 0 {
		parent := queue[0]
		queue = queue[1:]
		children := childrenMap[parent.Name()]
		for _, c := range children {
			queue = append(queue, c)
			if parent == systemToMerge.World() {
				err := sfs.AddFrame(c, attachFrame) // attach c to the attachFrame
				if err != nil {
					return err
				}
			} else {
				err := sfs.AddFrame(c, parent)
				if err != nil {
					return err
				}
			}
		}
	}

	// Copy flattening metadata from the merged system
	for componentName, model := range systemToMerge.flattenedModels {
		sfs.flattenedModels[componentName] = model
		sfs.componentSchemas[componentName] = systemToMerge.componentSchemas[componentName]
	}
	for frameName, mi := range systemToMerge.mimicFrames {
		sfs.mimicFrames[frameName] = mi
	}

	return nil
}

// FrameSystemSubset will take a frame system and a frame in that system, and return a new frame system rooted
// at the given frame and containing all descendents of it. The original frame system is unchanged.
//
// When the root is a SimpleModel with flattened internal frames, the subset is expanded to
// the component's origin frame so that sibling internal frames (and anything parented to
// them) are included in the subset.
func (sfs *FrameSystem) FrameSystemSubset(newRoot Frame) (*FrameSystem, error) {
	// If the root is a flattened SimpleModel, expand to the component's origin
	// frame so that sibling internal frames and their descendants are included.
	if _, isFlattenedComponent := sfs.flattenedModels[newRoot.Name()]; isFlattenedComponent {
		originName := sfs.parents[newRoot.Name()]
		if originFrame := sfs.Frame(originName); originFrame != nil {
			newRoot = originFrame
		}
	}

	newWorld := NewZeroStaticFrame(World)
	newFS := &FrameSystem{
		name:             newRoot.Name() + "_FS",
		world:            newWorld,
		frames:           map[string]Frame{},
		parents:          map[string]string{},
		flattenedModels:  map[string]*SimpleModel{},
		componentSchemas: map[string]*LinearInputsSchema{},
		mimicFrames:      map[string]*mimicInfo{},
	}

	rootFrame := sfs.Frame(newRoot.Name())
	if rootFrame == nil {
		return nil, NewFrameMissingError(newRoot.Name())
	}
	newFS.frames[newRoot.Name()] = newRoot
	newFS.parents[newRoot.Name()] = newWorld.Name()

	var traceParent func(Frame) bool
	traceParent = func(parent Frame) bool {
		// Determine to which frame system this frame and its parent should be added
		if parent == sfs.World() {
			// keep in sfs
			return false
		} else if parent == newRoot || newFS.frameExists(parent.Name()) {
			return true
		}
		parentName, exists := sfs.parents[parent.Name()]
		if !exists {
			return false
		}
		return traceParent(sfs.Frame(parentName))
	}

	// Deleting from a map as we iterate through it is OK and safe to do in Go
	for frameName, parentName := range sfs.parents {
		frame := sfs.Frame(frameName)
		parent := sfs.Frame(parentName)
		var addNew bool
		if parent == newRoot {
			addNew = true
		} else {
			addNew = traceParent(parent)
		}
		if addNew {
			newFS.frames[frame.Name()] = frame
			newFS.parents[frame.Name()] = parent.Name()
		}
	}

	// Copy flattening metadata for components whose frame exists in the subset
	for componentName, model := range sfs.flattenedModels {
		if newFS.frameExists(componentName) {
			newFS.flattenedModels[componentName] = model
			newFS.componentSchemas[componentName] = sfs.componentSchemas[componentName]
		}
	}
	for frameName, mi := range sfs.mimicFrames {
		if newFS.frameExists(frameName) {
			newFS.mimicFrames[frameName] = mi
		}
	}

	newFS.cachedBFSNames = bfsFrameNames(newFS)
	return newFS, nil
}

// DivideFrameSystem will take a frame system and a frame in that system, and return a new frame system rooted
// at the given frame and containing all descendents of it, while the original has the frame and its
// descendents removed. For example, if there is a frame system with two independent rovers, and one rover goes offline,
// A user could divide the frame system to remove the offline rover and have the rest of the frame system unaffected.
func (sfs *FrameSystem) DivideFrameSystem(newRoot Frame) (*FrameSystem, error) {
	newFS, err := sfs.FrameSystemSubset(newRoot)
	if err != nil {
		return nil, err
	}
	sfs.RemoveFrame(newRoot)
	return newFS, nil
}

// GetFrameToWorldTransform computes the position of src in the world frame based on inputMap.
func (sfs *FrameSystem) GetFrameToWorldTransform(inputs *LinearInputs, src Frame) (dualquat.Number, error) {
	ret := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}

	if !sfs.frameExists(src.Name()) {
		return ret, NewFrameMissingError(src.Name())
	}

	// If src is nil it is interpreted as the world frame
	var err error
	if src != nil {
		ret, err = sfs.composeTransforms(src, inputs)
		if err != nil {
			return ret, err
		}
	}
	return ret, err
}

// ReplaceFrame finds the original frame which shares its name with replacementFrame. We then transfer the original
// frame's children and parentage to replacementFrame. The original frame is removed entirely from the frame system.
// replacementFrame is not allowed to exist within the frame system at the time of the call.
func (sfs *FrameSystem) ReplaceFrame(replacementFrame Frame) error {
	var replaceMe Frame
	if replaceMe = sfs.Frame(replacementFrame.Name()); replaceMe == nil {
		return fmt.Errorf("%s not found in frame system", replacementFrame.Name())
	}
	if replaceMe == sfs.World() {
		return errors.New("cannot replace the World frame of a frame system")
	}

	// get replaceMe's raw parent (not the masked Parent() which hides internal frames)
	rawParentName, exists := sfs.parents[replaceMe.Name()]
	if !exists {
		return NewParentFrameNilError(replaceMe.Name())
	}
	replaceMeParent := sfs.Frame(rawParentName)

	// remove replaceMe from the frame system
	delete(sfs.frames, replaceMe.Name())
	delete(sfs.parents, replaceMe.Name())

	for frameName, parentName := range sfs.parents {
		// replace frame with parent as replaceMe with replaceWith
		if parentName == replaceMe.Name() {
			delete(sfs.parents, frameName)
			sfs.parents[frameName] = replacementFrame.Name()
		}
	}

	// add replacementFrame to frame system with parent of replaceMe
	if err := sfs.AddFrame(replacementFrame, replaceMeParent); err != nil {
		return err
	}

	// If replacing a flattened model's component, update the stored model
	if sm, ok := replacementFrame.(*SimpleModel); ok {
		if _, exists := sfs.flattenedModels[replacementFrame.Name()]; exists {
			sfs.flattenedModels[replacementFrame.Name()] = sm
		}
	}

	return nil
}

// Returns the relative pose between the parent and the destination frame.
func (sfs *FrameSystem) transformFromParent(inputs *LinearInputs, src, dst Frame) (spatial.DualQuaternion, error) {
	srcToWorld, err := sfs.GetFrameToWorldTransform(inputs, src)
	if err != nil {
		return spatial.DualQuaternion{}, err
	}

	if dst.Name() == World {
		return spatial.DualQuaternion{srcToWorld}, nil
	}

	dstToWorld, err := sfs.GetFrameToWorldTransform(inputs, dst)
	if err != nil {
		return spatial.DualQuaternion{}, err
	}

	// transform from source to world, world to target parent
	invA := spatial.DualQuaternion{Number: dualquat.ConjQuat(dstToWorld)}
	result := spatial.DualQuaternion{Number: invA.Transformation(srcToWorld)}
	return result, nil
}

// composeTransforms assumes there is one moveable frame and its DoF is equal to the `inputs`
// length.
func (sfs *FrameSystem) composeTransforms(frame Frame, linearInputs *LinearInputs) (dualquat.Number, error) {
	ret := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{},
	}

	numMoveableFrames := 0
	for sfs.parents[frame.Name()] != "" { // stop once you reach world node
		var pose spatial.Pose
		var err error

		if mi, isMimic := sfs.mimicFrames[frame.Name()]; isMimic {
			// Mimic frame: derive input from the source frame's input
			sourceInputs := linearInputs.Get(mi.sourceFrameName)
			if len(sourceInputs) == 0 {
				sourceInputs = sfs.resolveFrameInputs(linearInputs, mi.sourceFrameName)
			}
			if len(sourceInputs) == 0 {
				return ret, fmt.Errorf("mimic source frame %q has no inputs", mi.sourceFrameName)
			}
			derived := []Input{mi.multiplier*sourceInputs[0] + mi.offset}
			pose, err = frame.Transform(derived)
			if err != nil {
				return ret, err
			}
		} else if len(frame.DoF()) == 0 {
			pose, err = frame.Transform([]Input{})
			if err != nil {
				return ret, err
			}
		} else {
			frameInputs := linearInputs.Get(frame.Name())
			if frameInputs == nil {
				frameInputs = sfs.resolveFrameInputs(linearInputs, frame.Name())
			}
			numMoveableFrames++
			if len(frame.DoF()) != len(frameInputs) {
				return ret, NewIncorrectDoFError(len(frameInputs), len(frame.DoF()))
			}

			pose, err = frame.Transform(frameInputs)
			if err != nil {
				return ret, err
			}
		}

		ret = pose.(*spatial.DualQuaternion).Transformation(ret)
		frame = sfs.Frame(sfs.parents[frame.Name()])
	}

	return ret, nil
}

// MarshalJSON serializes a FrameSystem into JSON format.
func (sfs *FrameSystem) MarshalJSON() ([]byte, error) {
	type serializableFrameSystem struct {
		Name    string                     `json:"name"`
		World   json.RawMessage            `json:"world"`
		Frames  map[string]json.RawMessage `json:"frames"`
		Parents map[string]string          `json:"parents"`
	}
	worldFrameJSON, err := frameToJSON(sfs.World())
	if err != nil {
		return nil, err
	}

	typedFrames := make(map[string]json.RawMessage, 0)
	for name, frame := range sfs.frames {
		frameJSON, err := frameToJSON(frame)
		if err != nil {
			return nil, err
		}
		typedFrames[name] = frameJSON
	}
	serializedFS := serializableFrameSystem{
		Name:    sfs.name,
		World:   worldFrameJSON,
		Frames:  typedFrames,
		Parents: sfs.parents,
	}
	return json.Marshal(serializedFS)
}

// UnmarshalJSON parses a FrameSystem from JSON data.
// The flattened-model bookkeeping (flattenedModels, componentSchemas,
// mimicFrames) is not serialized. We regenerate it here via
// registerFlattenedModel, which is the same code path flattenModelIntoFS
// uses at construction.
func (sfs *FrameSystem) UnmarshalJSON(data []byte) error {
	type serializableFrameSystem struct {
		Name    string                     `json:"name"`
		World   json.RawMessage            `json:"world"`
		Frames  map[string]json.RawMessage `json:"frames"`
		Parents map[string]string          `json:"parents"`
	}
	var serFS serializableFrameSystem
	if err := json.Unmarshal(data, &serFS); err != nil {
		return err
	}

	worldFrame, err := jsonToFrame(serFS.World)
	if err != nil {
		return err
	}

	frameMap := make(map[string]Frame, len(serFS.Frames))
	for name, tF := range serFS.Frames {
		frame, err := jsonToFrame(tF)
		if err != nil {
			return err
		}
		frameMap[name] = frame
	}

	sfs.frames = frameMap
	sfs.parents = serFS.Parents
	sfs.world = worldFrame
	sfs.name = serFS.Name
	sfs.cachedBFSNames = bfsFrameNames(sfs)
	sfs.flattenedModels = map[string]*SimpleModel{}
	sfs.componentSchemas = map[string]*LinearInputsSchema{}
	sfs.mimicFrames = map[string]*mimicInfo{}
	for name, frame := range sfs.frames {
		sm := asFlattenableModel(frame)
		if sm == nil {
			continue
		}
		// Only regenerate flattened internals if the model was flattened into the outer FS at
		// construction time, i.e. its internal frames are present under the
		// namespaced names. A SimpleModel added directly via AddFrame has no
		// such internals and must not be registered.
		if wasFlattened(sfs, name, sm) {
			sfs.registerFlattenedModel(name, sm)
		}
	}
	return nil
}

// wasFlattened reports whether the model at componentName was flattened into
// the outer FS (its bfs-enumerated internal frames exist under the
// componentName:internalName convention).
func wasFlattened(sfs *FrameSystem, componentName string, model *SimpleModel) bool {
	internalNames := bfsFrameNames(model.internalFS)
	if len(internalNames) == 0 {
		return false
	}
	for _, internalName := range internalNames {
		if _, ok := sfs.frames[componentName+":"+internalName]; !ok {
			return false
		}
	}
	return true
}

// asFlattenableModel returns the underlying *SimpleModel if frame is (or wraps)
// a SimpleModel with at least one DoF, else nil. Zero-DoF models are not
// flattened (see addPartToFS).
func asFlattenableModel(frame Frame) *SimpleModel {
	switch f := frame.(type) {
	case *SimpleModel:
		if len(f.DoF()) > 0 {
			return f
		}
	case *namedFrame:
		return asFlattenableModel(f.Frame)
	}
	return nil
}

// NewZeroInputs returns a zeroed FrameSystemInputs ensuring all frames have inputs.
func NewZeroInputs(fs *FrameSystem) FrameSystemInputs {
	positions := make(FrameSystemInputs)
	for _, fn := range fs.FrameNames() {
		frame := fs.Frame(fn)
		if frame != nil {
			positions[fn] = make([]Input, len(frame.DoF()))
		}
	}
	return positions
}

// NewNeutralFrameSystemInputs returns a FrameSystemInputs ensuring all frames have inputs within their limits.
// It is similar to NewZeroInputs but the input values are clamped to be within their valid range.
// Zero is used when it falls within [min, max]; otherwise the nearest bound is chosen.
func NewNeutralFrameSystemInputs(fs *FrameSystem) FrameSystemInputs {
	inputs := make(FrameSystemInputs)
	for _, fn := range fs.FrameNames() {
		frame := fs.Frame(fn)
		if frame == nil {
			continue
		}
		dof := frame.DoF()
		if len(dof) == 0 {
			inputs[fn] = []Input{}
			continue
		}
		frameInputs := make([]Input, len(dof))
		for i, limit := range dof {
			frameInputs[i] = math.Max(limit.Min, math.Min(0, limit.Max))
		}
		inputs[fn] = frameInputs
	}
	return inputs
}

// NewZeroLinearInputs returns a zeroed LinearInputs ensuring all frames have inputs.
func NewZeroLinearInputs(fs *FrameSystem) *LinearInputs {
	positions := NewLinearInputs()
	for _, fn := range fs.FrameNames() {
		frame := fs.Frame(fn)
		if frame != nil {
			positions.Put(fn, make([]Input, len(frame.DoF())))
		}
	}
	return positions
}

// NewNeutralLinearInputs returns LinearInputs ensuring all frames have inputs within their limits.
// It is similar to NewZeroLinearInputs but the input values are clamped to be within their valid range.
// Zero is used when it falls within [min, max]; otherwise the nearest bound is chosen.
func NewNeutralLinearInputs(fs *FrameSystem) *LinearInputs {
	inputs := NewLinearInputs()
	for _, fn := range fs.FrameNames() {
		frame := fs.Frame(fn)
		if frame == nil {
			continue
		}
		dof := frame.DoF()
		if len(dof) == 0 {
			inputs.Put(fn, []Input{})
			continue
		}
		frameInputs := make([]Input, len(dof))
		for i, limit := range dof {
			frameInputs[i] = math.Max(limit.Min, math.Min(0, limit.Max))
		}
		inputs.Put(fn, frameInputs)
	}
	return inputs
}

// InterpolateFS interpolates.
func InterpolateFS(fs *FrameSystem, from, to *LinearInputs, by float64) (*LinearInputs, error) {
	interp := NewLinearInputs()
	for fn, fromInputs := range from.Items() {
		if len(fromInputs) == 0 {
			continue
		}

		frame := fs.Frame(fn)
		if frame == nil {
			return nil, NewFrameMissingError(fn)
		}

		toInputs := to.Get(fn)
		if toInputs == nil {
			return nil, fmt.Errorf("frame with name %s not found in `to` interpolation inputs", fn)
		}

		interpInputs, err := frame.Interpolate(fromInputs, toInputs, by)
		if err != nil {
			return nil, err
		}

		interp.Put(fn, interpInputs)
	}

	return interp, nil
}

// FrameSystemToPCD takes in a framesystem and returns a map where all elements are
// the point representation of their geometry type with respect to the world.
func FrameSystemToPCD(system *FrameSystem, inputs FrameSystemInputs, logger logging.Logger) (map[string][]r3.Vector, error) {
	vectorMap := make(map[string][]r3.Vector)
	geometriesInWorldFrame, err := FrameSystemGeometries(system, inputs)
	if err != nil {
		logger.Debug(err)
	}
	for _, geometries := range geometriesInWorldFrame {
		for _, geometry := range geometries.Geometries() {
			vectorMap[geometry.Label()] = geometry.ToPoints(defaultPointDensity)
		}
	}
	return vectorMap, nil
}

// FrameSystemGeometries takes in a framesystem and returns a map where all elements are
// GeometriesInFrames with a World reference frame. `FrameSystemGeometriesLinearInputs` is preferred
// for hot paths. This function is otherwise kept around for backwards compatibility.
func FrameSystemGeometries(fs *FrameSystem, inputMap FrameSystemInputs) (map[string]*GeometriesInFrame, error) {
	return FrameSystemGeometriesLinearInputs(fs, inputMap.ToLinearInputs())
}

// FrameSystemGeometriesLinearInputs takes in a framesystem and returns a LinearInputs where all
// elements are GeometriesInFrames with a World reference frame. This is preferred for hot
// paths. But requires the caller to manage a `LinearInputs`.
func FrameSystemGeometriesLinearInputs(fs *FrameSystem, linearInputs *LinearInputs) (map[string]*GeometriesInFrame, error) {
	var errAll error
	allGeometries := make(map[string]*GeometriesInFrame, 0)
	for _, name := range fs.FrameNames() {
		frame := fs.Frame(name)
		inputs, err := linearInputs.GetFrameInputs(frame)
		if err != nil {
			errAll = multierr.Append(errAll, err)
			continue
		}

		geosInFrame, err := frame.Geometries(inputs)
		if err != nil {
			errAll = multierr.Append(errAll, err)
			continue
		}

		if len(geosInFrame.Geometries()) > 0 {
			transformed, err := fs.Transform(linearInputs, geosInFrame, World)
			if err != nil {
				errAll = multierr.Append(errAll, err)
				continue
			}
			allGeometries[name] = transformed.(*GeometriesInFrame)
		}
	}

	return allGeometries, errAll
}

// ToProtobuf turns all the interfaces into serializable types.
func (part *FrameSystemPart) ToProtobuf() (*pb.FrameSystemConfig, error) {
	if part.FrameConfig == nil {
		return nil, ErrNoModelInformation
	}
	linkFrame, err := LinkInFrameToTransformProtobuf(part.FrameConfig)
	if err != nil {
		return nil, err
	}
	var modelJSON SimpleModel
	if part.ModelFrame != nil {
		bytes, err := part.ModelFrame.MarshalJSON()
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(bytes, &modelJSON)
		if err != nil {
			return nil, err
		}
	}
	kinematics, err := protoutils.StructToStructPb(modelJSON.modelConfig)
	if err != nil {
		return nil, err
	}
	return &pb.FrameSystemConfig{
		Frame:      linkFrame,
		Kinematics: kinematics,
	}, nil
}

// ProtobufToFrameSystemPart takes a protobuf object and transforms it into a FrameSystemPart.
func ProtobufToFrameSystemPart(fsc *pb.FrameSystemConfig) (*FrameSystemPart, error) {
	frameConfig, err := LinkInFrameFromTransformProtobuf(fsc.Frame)
	if err != nil {
		return nil, err
	}
	part := &FrameSystemPart{
		FrameConfig: frameConfig,
	}

	if len(fsc.Kinematics.AsMap()) > 0 {
		modelBytes, err := json.Marshal(fsc.Kinematics.AsMap())
		if err != nil {
			return nil, err
		}
		modelFrame, err := UnmarshalModelJSON(modelBytes, frameConfig.Name())
		if err != nil {
			if errors.Is(err, ErrNoModelInformation) {
				return part, nil
			}
			return nil, err
		}
		part.ModelFrame = modelFrame
	}
	return part, nil
}

// LinkInFrameToFrameSystemPart creates a FrameSystem part out of a PoseInFrame.
func LinkInFrameToFrameSystemPart(transform *LinkInFrame) (*FrameSystemPart, error) {
	if transform.Name() == "" || transform.Parent() == "" {
		return nil, ErrEmptyStringFrameName
	}
	part := &FrameSystemPart{
		FrameConfig: transform,
	}
	return part, nil
}

// createFramesFromPart will gather the frame information and build the frames from the given robot part.
func createFramesFromPart(part *FrameSystemPart) (Frame, Frame, error) {
	if part == nil || part.FrameConfig == nil {
		return nil, nil, errors.New("config for FrameSystemPart is nil")
	}

	var modelFrame Frame
	// use identity frame if no model frame defined
	if part.ModelFrame == nil {
		modelFrame = NewZeroStaticFrame(part.FrameConfig.Name())
	} else {
		if part.ModelFrame.Name() != part.FrameConfig.Name() {
			modelFrame = NewNamedFrame(part.ModelFrame, part.FrameConfig.Name())
		} else {
			modelFrame = part.ModelFrame
		}
	}
	// staticOriginFrame defines a change in origin from the parent part.
	// If it is empty, the new frame will have the same origin as the parent.
	staticOriginFrame, err := part.FrameConfig.ToStaticFrame(part.FrameConfig.Name() + "_origin")
	if err != nil {
		return nil, nil, err
	}

	// If the user has specified a geometry, and the model is a zero DOF frame (e.g. a gripper), we want to overwrite the geometry
	// with the user-supplied one without changing the model transform
	if len(modelFrame.DoF()) == 0 {
		offsetGeom, err := staticOriginFrame.Geometries([]Input{})
		if err != nil {
			return nil, nil, err
		}
		pose, err := modelFrame.Transform([]Input{})
		if err != nil {
			return nil, nil, err
		}
		if len(offsetGeom.Geometries()) > 0 {
			// If there are offset geometries, they should replace the static geometries, so the static frame is recreated with no geoms
			noGeomFrame, err := NewStaticFrame(modelFrame.Name(), pose)
			if err != nil {
				return nil, nil, err
			}
			modelFrame = noGeomFrame
		}
	}

	// Since the geometry of a frame system part is intended to be located at the origin of the model frame, we place it post-transform
	// in the "_origin" static frame
	return modelFrame, &tailGeometryStaticFrame{staticOriginFrame.(*staticFrame)}, nil
}

// TopologicallySortParts takes a potentially un-ordered slice of frame system parts and sorts them,
// beginning at the world node. The world frame is not included in the output.
//
// Parts that are missing a parent will be ignored and returned as part of the second return
// value. If it's important that all inputs are connected to the world frame, a caller must
// conditionally error on that second return value.
//
// Given each node can only have one parent, and we always return the tree rooted at the world
// frame, cycles are impossible. The "unlinked" second return value might be unlinked because a
// parent does not exist, or the nodes are in a cycle with each other.
func TopologicallySortParts(parts []*FrameSystemPart) ([]*FrameSystemPart, []*FrameSystemPart) {
	// set up directory to check existence of parents
	partNameIndex := make(map[string]bool, len(parts))
	partNameIndex[World] = true
	for _, part := range parts {
		partNameIndex[part.FrameConfig.Name()] = true
	}

	// make map of children
	children := make(map[string][]*FrameSystemPart)
	for _, part := range parts {
		parent := part.FrameConfig.Parent()
		if !partNameIndex[parent] {
			continue
		}

		children[parent] = append(children[parent], part)
	}

	// If there are no frames, return the empty list
	if len(children) == 0 {
		return nil, parts
	}

	queue := make([]string, 0)
	visited := make(map[string]bool)
	topoSortedParts := []*FrameSystemPart{}
	queue = append(queue, World)
	// begin adding frames to tree
	for len(queue) != 0 {
		parent := queue[0]
		queue = queue[1:]
		if visited[parent] {
			return nil, nil
		}

		visited[parent] = true
		sort.Slice(children[parent], func(i, j int) bool {
			return children[parent][i].FrameConfig.Name() < children[parent][j].FrameConfig.Name()
		}) // sort alphabetically within the topological sort

		for _, part := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			queue = append(queue, part.FrameConfig.Name())
			topoSortedParts = append(topoSortedParts, part)
		}
	}

	unlinkedParts := make([]*FrameSystemPart, 0, 4)
	for _, part := range parts {
		if !visited[part.FrameConfig.Name()] {
			unlinkedParts = append(unlinkedParts, part)
		}
	}

	return topoSortedParts, unlinkedParts
}

// flattenModelIntoFS unpacks a SimpleModel's internal frames into the outer FrameSystem.
// Each internal frame is renamed with a namespace prefix (componentName:internalName).
// Mimic joint info is stored in the FS's mimicFrames map for use by composeTransforms.
func flattenModelIntoFS(outerFS *FrameSystem, model *SimpleModel, componentName string, attachTo Frame) error {
	internalFS := model.internalFS
	internalNames := bfsFrameNames(internalFS)

	for _, internalName := range internalNames {
		namespacedName := componentName + ":" + internalName
		innerFrame := internalFS.Frame(internalName)
		if innerFrame == nil {
			return NewFrameMissingError(internalName)
		}

		// Determine the parent in the outer FS
		internalParentName := internalFS.parents[internalName]
		var parentFrame Frame
		if internalParentName == World {
			parentFrame = attachTo
		} else {
			namespacedParent := componentName + ":" + internalParentName
			parentFrame = outerFS.Frame(namespacedParent)
			if parentFrame == nil {
				return NewFrameMissingError(namespacedParent)
			}
		}

		wrappedFrame := NewNamedFrame(innerFrame, namespacedName)
		if err := outerFS.AddFrame(wrappedFrame, parentFrame); err != nil {
			return err
		}
	}

	outerFS.registerFlattenedModel(componentName, model)
	return nil
}

// registerFlattenedModel populates the three derived maps (flattenedModels,
// componentSchemas, mimicFrames) for a flattened model. The namespaced
// componentName:* frames must already exist in sfs.frames so the schema metas
// can be bound to them. Called from both flattenModelIntoFS and UnmarshalJSON
func (sfs *FrameSystem) registerFlattenedModel(componentName string, model *SimpleModel) {
	sfs.flattenedModels[componentName] = model

	for internalName, mm := range model.mimicMappings {
		if mm == nil {
			continue
		}
		sfs.mimicFrames[componentName+":"+internalName] = &mimicInfo{
			sourceFrameName: componentName + ":" + mm.sourceFrameName,
			multiplier:      mm.valueMultiplier,
			offset:          mm.valueOffset,
		}
	}

	namespacedMetas := make([]linearInputMeta, 0, len(model.inputSchema.metas))
	for _, meta := range model.inputSchema.metas {
		namespacedMetas = append(namespacedMetas, linearInputMeta{
			frameName: componentName + ":" + meta.frameName,
			offset:    meta.offset,
			dof:       meta.dof,
			frame:     sfs.Frame(componentName + ":" + meta.frameName),
		})
	}
	sfs.componentSchemas[componentName] = &LinearInputsSchema{metas: namespacedMetas}
}

// bfsFrameNames returns frame names in BFS order from world. Children at each level are
// sorted alphabetically for determinism.
func bfsFrameNames(fs *FrameSystem) []string {
	childrenOf := map[string][]string{}
	for name := range fs.frames {
		// Use raw parents map to preserve internal frame structure (Parent() masks internal frames).
		rawParentName, exists := fs.parents[name]
		if !exists {
			continue
		}
		childrenOf[rawParentName] = append(childrenOf[rawParentName], name)
	}
	for k := range childrenOf {
		sort.Strings(childrenOf[k])
	}

	var result []string
	queue := []string{World}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur != World {
			result = append(result, cur)
		}
		queue = append(queue, childrenOf[cur]...)
	}
	return result
}

// cloneFrameSystem creates a deep copy of a FrameSystem by cloning each frame individually
// and rebuilding the parent-child relationships.
func cloneFrameSystem(fs *FrameSystem) (*FrameSystem, error) {
	newFS := NewEmptyFrameSystem(fs.name)
	for _, name := range fs.FrameNames() {
		frame := fs.Frame(name)
		clonedFrame, err := clone(frame)
		if err != nil {
			return nil, fmt.Errorf("cloning frame %q: %w", name, err)
		}
		parent, err := fs.Parent(frame)
		if err != nil {
			return nil, err
		}
		if err := newFS.AddFrame(clonedFrame, newFS.Frame(parent.Name())); err != nil {
			return nil, err
		}
	}
	return newFS, nil
}

func frameSystemsAlmostEqual(fs1, fs2 *FrameSystem, epsilon float64) (bool, error) {
	if fs1 == nil {
		return fs2 == nil, nil
	} else if fs2 == nil {
		return false, nil
	}

	if fs1.Name() != fs2.Name() {
		return false, nil
	}

	worldFrameEquality, err := framesAlmostEqual(fs1.World(), fs2.World(), epsilon)
	if err != nil {
		return false, err
	}
	if !worldFrameEquality {
		return false, nil
	}

	if !reflect.DeepEqual(fs1.parents, fs2.parents) {
		return false, nil
	}

	if len(fs1.FrameNames()) != len(fs2.FrameNames()) {
		return false, nil
	}
	for frameName, frame := range fs1.frames {
		frame2, ok := fs2.frames[frameName]
		if !ok {
			return false, nil
		}
		frameEquality, err := framesAlmostEqual(frame, frame2, epsilon)
		if err != nil {
			return false, err
		}
		if !frameEquality {
			return false, nil
		}
	}
	return true, nil
}
