package referenceframe

import (
	"strconv"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

// WorldState is a struct to store the data representation of the robot's environment.
type WorldState struct {
	obstacleNames map[string]bool
	unnamedCount  int
	obstacles     []*GeometriesInFrame
	transforms    []*LinkInFrame
}

const unnamedWorldStateGeometryPrefix = "unnamedWorldStateGeometry_"

// NewEmptyWorldState is a constructor for an empty WorldState struct.
func NewEmptyWorldState() *WorldState {
	return &WorldState{
		obstacleNames: make(map[string]bool),
		obstacles:     make([]*GeometriesInFrame, 0),
		transforms:    make([]*LinkInFrame, 0),
	}
}

// WorldStateFromProtobuf takes the protobuf definition of a WorldState and converts it to a rdk defined WorldState.
func WorldStateFromProtobuf(proto *commonpb.WorldState) (*WorldState, error) {
	if proto == nil {
		return NewEmptyWorldState(), nil
	}

	transforms, err := LinkInFramesFromTransformsProtobuf(proto.GetTransforms())
	if err != nil {
		return nil, err
	}

	ws := &WorldState{transforms: transforms}
	for _, protoGeometries := range proto.GetObstacles() {
		geometries, err := ProtobufToGeometriesInFrame(protoGeometries)
		if err != nil {
			return nil, err
		}
		if err = ws.AddObstacles(geometries.frame, geometries.geometries...); err != nil {
			return nil, err
		}
	}

	return ws, nil
}

// AddObstacles takes in a list of geometries and a frame corresponding with the reference frame associated with them and adds them
// as obstacles to the worldState.
func (ws *WorldState) AddObstacles(frame string, geometries ...spatialmath.Geometry) error {
	geometries, err := ws.rectifyNames(geometries)
	if err != nil {
		return err
	}
	ws.obstacles = append(ws.obstacles, NewGeometriesInFrame(frame, geometries))
	return nil
}

// ObstacleNames returns the set of geometry names that have been registered in the WorldState, represented as a map.
func (ws *WorldState) ObstacleNames() map[string]bool {
	return ws.obstacleNames
}

// AddTransforms adds the given transforms to the WorldState.
func (ws *WorldState) AddTransforms(transforms ...*LinkInFrame) {
	ws.transforms = append(ws.transforms, transforms...)
}

// Transforms returns the transforms that have been added to the WorldState.
func (ws *WorldState) Transforms() []*LinkInFrame {
	return ws.transforms
}

// WorldStateToProtobuf takes an rdk WorldState and converts it to the protobuf definition of a WorldState.
func WorldStateToProtobuf(worldState *WorldState) (*commonpb.WorldState, error) {
	convertGeometriesToProto := func(allGeometries []*GeometriesInFrame) []*commonpb.GeometriesInFrame {
		list := make([]*commonpb.GeometriesInFrame, 0, len(allGeometries))
		for _, geometries := range allGeometries {
			list = append(list, GeometriesInFrameToProtobuf(geometries))
		}
		return list
	}

	transforms, err := LinkInFramesToTransformsProtobuf(worldState.transforms)
	if err != nil {
		return nil, err
	}

	return &commonpb.WorldState{
		Obstacles:  convertGeometriesToProto(worldState.obstacles),
		Transforms: transforms,
	}, nil
}

// ObstaclesInWorldFrame takes a frame system and a set of inputs for that frame system and converts all the obstacles
// in the WorldState such that they are in the frame system's World reference frame.
func (ws *WorldState) ObstaclesInWorldFrame(fs FrameSystem, inputs map[string][]Input) (*GeometriesInFrame, error) {
	if ws == nil {
		ws = NewEmptyWorldState()
	}

	allGeometries := make([]spatialmath.Geometry, 0, len(ws.obstacles))
	for _, gf := range ws.obstacles {
		tf, err := fs.Transform(inputs, gf, World)
		if err != nil {
			return nil, err
		}
		allGeometries = append(allGeometries, tf.(*GeometriesInFrame).Geometries()...)
	}
	return NewGeometriesInFrame(World, allGeometries), nil
}

func (ws *WorldState) rectifyNames(geometries []spatialmath.Geometry) ([]spatialmath.Geometry, error) {
	checkedGeometries := make([]spatialmath.Geometry, len(geometries))
	for i, geometry := range geometries {
		name := geometry.Label()
		if name == "" {
			name = unnamedWorldStateGeometryPrefix + strconv.Itoa(ws.unnamedCount)
			geometry.SetLabel(name)
			ws.unnamedCount++
		}

		if _, present := ws.obstacleNames[name]; present {
			return nil, NewDuplicateGeometryNameError(name)
		}
		ws.obstacleNames[name] = true
		checkedGeometries[i] = geometry
	}
	return checkedGeometries, nil
}
