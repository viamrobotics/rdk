package referenceframe

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/logging"
	spatial "go.viam.com/rdk/spatialmath"
)

// World is the string "world", but made into an exported constant.
const World = "world"

// defaultPointDensity ensures we use the default value specified within the spatialmath package.
const defaultPointDensity = 0.

// FrameSystem represents a tree of frames connected to each other, allowing for transformations between any two frames.
type FrameSystem interface {
	// Name returns the name of this FrameSystem
	Name() string

	// World returns the frame corresponding to the root of the FrameSystem, from which other frames are defined with respect to
	World() Frame

	// FrameNames returns the names of all of the frames that exist in the FrameSystem
	FrameNames() []string

	// Frame returns the Frame in the FrameSystem corresponding to
	Frame(name string) Frame

	// AddFrame inserts a given Frame into the FrameSystem as a child of the parent Frame
	AddFrame(frame, parent Frame) error

	// RemoveFrame removes the given Frame from the FrameSystem
	RemoveFrame(frame Frame)

	// TracebackFrame traces the parentage of the given frame up to the world, and returns the full list of frames in between.
	// The list will include both the query frame and the world referenceframe
	TracebackFrame(frame Frame) ([]Frame, error)

	// Parent returns the parent Frame for the given Frame in the FrameSystem
	Parent(frame Frame) (Frame, error)

	// Transform takes in a Transformable object and destination frame, and returns the pose from the first to the second. Positions
	// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
	Transform(positions map[string][]Input, object Transformable, dst string) (Transformable, error)

	// FrameSystemSubset will take a frame system and a frame in that system, and return a new frame system rooted
	// at the given frame and containing all descendents of it. The original frame system is unchanged.
	FrameSystemSubset(newRoot Frame) (FrameSystem, error)

	// DivideFrameSystem will take a frame system and a frame in that system, and return a new frame system rooted
	// at the given frame and containing all descendents of it, while the original has the frame and its
	// descendents removed.
	DivideFrameSystem(newRoot Frame) (FrameSystem, error)

	// MergeFrameSystem combines two frame systems together, placing the world of systemToMerge at the attachTo frame in the frame system
	MergeFrameSystem(systemToMerge FrameSystem, attachTo Frame) error

	// ReplaceFrame finds the original frame which shares its name with replacementFrame. We then transfer the original
	// frame's children and parentage to replacementFrame. The original frame is removed entirely from the frame system.
	// replacementFrame is not allowed to exist within the frame system at the time of the call.
	ReplaceFrame(replacementFrame Frame) error
}

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// FrameConfig gives the frame's location relative to parent,
// and ModelFrame is an optional ModelJSON that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	FrameConfig *LinkInFrame
	ModelFrame  Model
}

// simpleFrameSystem implements FrameSystem. It is a simple tree graph.
type simpleFrameSystem struct {
	name    string
	world   Frame // separate from the map of frames so it can be detached easily
	frames  map[string]Frame
	parents map[Frame]Frame
}

// NewEmptyFrameSystem creates a graph of Frames that have.
func NewEmptyFrameSystem(name string) FrameSystem {
	worldFrame := NewZeroStaticFrame(World)
	return &simpleFrameSystem{name, worldFrame, map[string]Frame{}, map[Frame]Frame{}}
}

// NewFrameSystem assembles a frame system from a set of parts and additional transforms.
func NewFrameSystem(name string, parts []*FrameSystemPart, additionalTransforms []*LinkInFrame) (FrameSystem, error) {
	allParts := make([]*FrameSystemPart, 0, len(parts)+len(additionalTransforms))
	allParts = append(allParts, parts...)
	for _, tf := range additionalTransforms {
		transformPart, err := LinkInFrameToFrameSystemPart(tf)
		if err != nil {
			return nil, err
		}
		allParts = append(allParts, transformPart)
	}

	// ensure that at least one frame connects to world if the frame system is not empty
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
	// Topologically sort parts
	sortedParts, err := TopologicallySortParts(allParts)
	if err != nil {
		return nil, err
	}
	if len(sortedParts) != len(allParts) {
		return nil, errors.Errorf(
			"frame system has disconnected frames. connected frames: %v, all frames: %v",
			getPartNames(sortedParts),
			getPartNames(allParts),
		)
	}
	fs := NewEmptyFrameSystem(name)
	for _, part := range sortedParts {
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := createFramesFromPart(part)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		if err = fs.AddFrame(staticOffsetFrame, fs.Frame(part.FrameConfig.Parent())); err != nil {
			return nil, err
		}
		if err = fs.AddFrame(modelFrame, staticOffsetFrame); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

// World returns the base world referenceframe.
func (sfs *simpleFrameSystem) World() Frame {
	return sfs.world
}

// Parent returns the parent frame of the input referenceframe. nil if input is World.
func (sfs *simpleFrameSystem) Parent(frame Frame) (Frame, error) {
	if !sfs.frameExists(frame.Name()) {
		return nil, NewFrameMissingError(frame.Name())
	}
	if parent := sfs.parents[frame]; parent != nil {
		return parent, nil
	}
	return nil, NewParentFrameNilError(frame.Name())
}

// frameExists is a helper function to see if a frame with a given name already exists in the system.
func (sfs *simpleFrameSystem) frameExists(name string) bool {
	if name == World {
		return true
	}
	if _, ok := sfs.frames[name]; ok {
		return true
	}
	return false
}

// RemoveFrame will delete the given frame and all descendents from the frame system if it exists.
func (sfs *simpleFrameSystem) RemoveFrame(frame Frame) {
	delete(sfs.frames, frame.Name())
	delete(sfs.parents, frame)

	// Remove all descendents
	for f, parent := range sfs.parents {
		if parent == frame {
			sfs.RemoveFrame(f)
		}
	}
}

// Frame returns the frame given the name of the referenceframe. Returns nil if the frame is not found.
func (sfs *simpleFrameSystem) Frame(name string) Frame {
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
func (sfs *simpleFrameSystem) TracebackFrame(query Frame) ([]Frame, error) {
	if !sfs.frameExists(query.Name()) {
		return nil, NewFrameMissingError(query.Name())
	}
	if query == sfs.world {
		return []Frame{query}, nil
	}
	parents, err := sfs.TracebackFrame(sfs.parents[query])
	if err != nil {
		return nil, err
	}
	return append([]Frame{query}, parents...), nil
}

// FrameNames returns the list of frame names registered in the frame system.
func (sfs *simpleFrameSystem) FrameNames() []string {
	var frameNames []string
	for k := range sfs.frames {
		frameNames = append(frameNames, k)
	}
	return frameNames
}

// AddFrame sets an already defined Frame into the system.
func (sfs *simpleFrameSystem) AddFrame(frame, parent Frame) error {
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
	sfs.parents[frame] = parent
	return nil
}

// Transform takes in a Transformable object and destination frame, and returns the pose from the first to the second. Positions
// is a map of inputs for any frames with non-zero DOF, with slices of inputs keyed to the frame name.
func (sfs *simpleFrameSystem) Transform(positions map[string][]Input, object Transformable, dst string) (Transformable, error) {
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

	var tfParent *PoseInFrame
	var err error
	if _, ok := object.(*GeometriesInFrame); ok && src != World {
		// We don't want to apply the final transformation when that is taken care of by the geometries
		// This has to do with the way we decided to tie geometries to frames for ease of defining them in the model_json file
		// A frame is assigned a pose and a geometry and the two are not coupled together. This way you do can define everything relative
		// to the parent frame. So geometries are tied to the frame they are assigned to but we do not want to actually transform them
		// along the final transformation.
		tfParent, err = sfs.transformFromParent(positions, sfs.parents[srcFrame], sfs.Frame(dst))
	} else {
		tfParent, err = sfs.transformFromParent(positions, srcFrame, sfs.Frame(dst))
	}
	if err != nil {
		return nil, err
	}
	return object.Transform(tfParent), nil
}

// Name returns the name of the simpleFrameSystem.
func (sfs *simpleFrameSystem) Name() string {
	return sfs.name
}

// MergeFrameSystem will combine two frame systems together, placing the world of systemToMerge at the "attachTo" frame in sfs.
// The frame where systemToMerge will be attached to must already exist within sfs, so should be added before Merge happens.
// Merging is necessary when including remote robots, dynamically building systems of robots, or mutating a robot after it
// has already been initialized. For example, two independent rovers, each with their own frame system, need to now know where
// they are in relation to each other and need to have their frame systems combined.
func (sfs *simpleFrameSystem) MergeFrameSystem(systemToMerge FrameSystem, attachTo Frame) error {
	attachFrame := sfs.Frame(attachTo.Name())
	if attachFrame == nil {
		return NewFrameMissingError(attachTo.Name())
	}

	// make a map where the parent frame is the key and the slice of children frames is the value
	childrenMap := map[Frame][]Frame{}
	for _, name := range systemToMerge.FrameNames() {
		child := systemToMerge.Frame(name)
		parent, err := systemToMerge.Parent(child)
		if err != nil {
			if errors.Is(err, NewParentFrameNilError(child.Name())) {
				continue
			}
			return err
		}
		childrenMap[parent] = append(childrenMap[parent], child)
	}

	// add every frame from systemToMerge to the base frame system.
	queue := []Frame{systemToMerge.World()}
	for len(queue) != 0 {
		parent := queue[0]
		queue = queue[1:]
		children := childrenMap[parent]
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
	return nil
}

// FrameSystemSubset will take a frame system and a frame in that system, and return a new frame system rooted
// at the given frame and containing all descendents of it. The original frame system is unchanged.
func (sfs *simpleFrameSystem) FrameSystemSubset(newRoot Frame) (FrameSystem, error) {
	newWorld := NewZeroStaticFrame(World)
	newFS := &simpleFrameSystem{newRoot.Name() + "_FS", newWorld, map[string]Frame{}, map[Frame]Frame{}}

	rootFrame := sfs.Frame(newRoot.Name())
	if rootFrame == nil {
		return nil, NewFrameMissingError(newRoot.Name())
	}
	newFS.frames[newRoot.Name()] = newRoot
	newFS.parents[newRoot] = newWorld

	var traceParent func(Frame) bool
	traceParent = func(parent Frame) bool {
		// Determine to which frame system this frame and its parent should be added
		if parent == sfs.World() {
			// keep in sfs
			return false
		} else if parent == newRoot || newFS.frameExists(parent.Name()) {
			return true
		}
		return traceParent(sfs.parents[parent])
	}

	// Deleting from a map as we iterate through it is OK and safe to do in Go
	for frame, parent := range sfs.parents {
		var addNew bool
		if parent == newRoot {
			addNew = true
		} else {
			addNew = traceParent(parent)
		}
		if addNew {
			newFS.frames[frame.Name()] = frame
			newFS.parents[frame] = parent
		}
	}

	return newFS, nil
}

// DivideFrameSystem will take a frame system and a frame in that system, and return a new frame system rooted
// at the given frame and containing all descendents of it, while the original has the frame and its
// descendents removed. For example, if there is a frame system with two independent rovers, and one rover goes offline,
// A user could divide the frame system to remove the offline rover and have the rest of the frame system unaffected.
func (sfs *simpleFrameSystem) DivideFrameSystem(newRoot Frame) (FrameSystem, error) {
	newFS, err := sfs.FrameSystemSubset(newRoot)
	if err != nil {
		return nil, err
	}
	sfs.RemoveFrame(newRoot)
	return newFS, nil
}

func (sfs *simpleFrameSystem) getFrameToWorldTransform(inputMap map[string][]Input, src Frame) (spatial.Pose, error) {
	if !sfs.frameExists(src.Name()) {
		return nil, NewFrameMissingError(src.Name())
	}

	// If src is nil it is interpreted as the world frame
	var err error
	srcToWorld := spatial.NewZeroPose()
	if src != nil {
		srcToWorld, err = sfs.composeTransforms(src, inputMap)
		if err != nil && srcToWorld == nil {
			return nil, err
		}
	}
	return srcToWorld, err
}

// ReplaceFrame finds the original frame which shares its name with replacementFrame. We then transfer the original
// frame's children and parentage to replacementFrame. The original frame is removed entirely from the frame system.
// replacementFrame is not allowed to exist within the frame system at the time of the call.
func (sfs *simpleFrameSystem) ReplaceFrame(replacementFrame Frame) error {
	var replaceMe Frame
	if replaceMe = sfs.Frame(replacementFrame.Name()); replaceMe == nil {
		return fmt.Errorf("%s not found in frame system", replacementFrame.Name())
	}
	if replaceMe == sfs.World() {
		return errors.New("cannot replace the World frame of a frame system")
	}

	// get replaceMe's parent
	replaceMeParent, err := sfs.Parent(replaceMe)
	if err != nil {
		return err
	}

	// remove replaceMe from the frame system
	delete(sfs.frames, replaceMe.Name())
	delete(sfs.parents, replaceMe)

	for f, parent := range sfs.parents {
		// replace frame with parent as replaceMe with replaceWith
		if parent == replaceMe {
			delete(sfs.parents, f)
			sfs.parents[f] = replacementFrame
		}
	}
	// add replacementFrame to frame system with parent of replaceMe
	return sfs.AddFrame(replacementFrame, replaceMeParent)
}

// Returns the relative pose between the parent and the destination frame.
func (sfs *simpleFrameSystem) transformFromParent(inputMap map[string][]Input, src, dst Frame) (*PoseInFrame, error) {
	// catch all errors together to allow for hypothetical calculations that result in errors
	var errAll error
	dstToWorld, err := sfs.getFrameToWorldTransform(inputMap, dst)
	multierr.AppendInto(&errAll, err)
	srcToWorld, err := sfs.getFrameToWorldTransform(inputMap, src)
	multierr.AppendInto(&errAll, err)
	if errAll != nil && (dstToWorld == nil || srcToWorld == nil) {
		return nil, errAll
	}

	// transform from source to world, world to target parent
	return NewPoseInFrame(dst.Name(), spatial.PoseBetween(dstToWorld, srcToWorld)), nil
}

// compose the quaternions from the input frame to the world referenceframe.
func (sfs *simpleFrameSystem) composeTransforms(frame Frame, inputMap map[string][]Input) (spatial.Pose, error) {
	q := spatial.NewZeroPose() // empty initial dualquat
	var errAll error
	for sfs.parents[frame] != nil { // stop once you reach world node
		// Transform() gives FROM q TO parent. Add new transforms to the left.
		pose, err := poseFromPositions(frame, inputMap)
		if err != nil && pose == nil {
			return nil, err
		}
		multierr.AppendInto(&errAll, err)
		q = spatial.Compose(pose, q)
		frame = sfs.parents[frame]
	}
	return q, errAll
}

// StartPositions returns a zeroed input map ensuring all frames have inputs.
func StartPositions(fs FrameSystem) map[string][]Input {
	positions := make(map[string][]Input)
	for _, fn := range fs.FrameNames() {
		frame := fs.Frame(fn)
		if frame != nil {
			positions[fn] = make([]Input, len(frame.DoF()))
		}
	}
	return positions
}

// FrameSystemToPCD takes in a framesystem and returns a map where all elements are
// the point representation of their geometry type with respect to the world.
func FrameSystemToPCD(system FrameSystem, inputs map[string][]Input, logger logging.Logger) (map[string][]r3.Vector, error) {
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

// FrameSystemGeometries takes in a framesystem and returns a map where all elements are GeometriesInFrames with a World reference frame.
func FrameSystemGeometries(fs FrameSystem, inputMap map[string][]Input) (map[string]*GeometriesInFrame, error) {
	var errAll error
	allGeometries := make(map[string]*GeometriesInFrame, 0)
	for _, name := range fs.FrameNames() {
		frame := fs.Frame(name)
		inputs, err := GetFrameInputs(frame, inputMap)
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
			transformed, err := fs.Transform(inputMap, geosInFrame, World)
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
	var modelJSON map[string]interface{}
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
	kinematics, err := protoutils.StructToStructPb(modelJSON)
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
	var err error
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
	staticOriginName := part.FrameConfig.Name() + "_origin"
	// By default, this
	originFrame, err := part.FrameConfig.ToStaticFrame(staticOriginName)
	if err != nil {
		return nil, nil, err
	}
	staticOriginFrame, ok := originFrame.(*staticFrame)
	if !ok {
		return nil, nil, errors.New("failed to cast originFrame to a static frame")
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
	return modelFrame, &tailGeometryStaticFrame{staticOriginFrame}, nil
}

// Names returns the names of input parts.
func getPartNames(parts []*FrameSystemPart) []string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.FrameConfig.Name()
	}
	return names
}

// TopologicallySortParts takes a potentially un-ordered slice of frame system parts and
// sorts them, beginning at the world node.
func TopologicallySortParts(parts []*FrameSystemPart) ([]*FrameSystemPart, error) {
	// set up directory to check existence of parents
	existingParts := make(map[string]bool, len(parts))
	existingParts[World] = true
	for _, part := range parts {
		existingParts[part.FrameConfig.Name()] = true
	}
	// make map of children
	children := make(map[string][]*FrameSystemPart)
	for _, part := range parts {
		parent := part.FrameConfig.Parent()
		if !existingParts[parent] {
			return nil, NewParentFrameMissingError(part.FrameConfig.Name(), parent)
		}
		children[part.FrameConfig.Parent()] = append(children[part.FrameConfig.Parent()], part)
	}
	topoSortedParts := []*FrameSystemPart{} // keep track of tree structure
	// If there are no frames, return the empty list
	if len(children) == 0 {
		return topoSortedParts, nil
	}
	stack := make([]string, 0)
	visited := make(map[string]bool)
	if _, ok := children[World]; !ok {
		return nil, ErrNoWorldConnection
	}
	stack = append(stack, World)
	// begin adding frames to tree
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, errors.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		sort.Slice(children[parent], func(i, j int) bool {
			return children[parent][i].FrameConfig.Name() < children[parent][j].FrameConfig.Name()
		}) // sort alphabetically within the topological sort
		for _, part := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, part.FrameConfig.Name())
			topoSortedParts = append(topoSortedParts, part)
		}
	}
	return topoSortedParts, nil
}

func poseFromPositions(frame Frame, positions map[string][]Input) (spatial.Pose, error) {
	inputs, err := GetFrameInputs(frame, positions)
	if err != nil {
		return nil, err
	}
	return frame.Transform(inputs)
}
