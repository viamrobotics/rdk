package referenceframe

import (
	"fmt"
	"strconv"

	commonpb "go.viam.com/api/common/v1"

	spatial "go.viam.com/rdk/spatialmath"
)

// WorldState is a struct to store the data representation of the robot's environment.
type WorldState struct {
	Obstacles  []*GeometriesInFrame
	Transforms []*LinkInFrame
}

const unnamedWorldStateGeometryPrefix = "unnamedWorldStateGeometry_"

// WorldStateFromProtobuf takes the protobuf definition of a WorldState and converts it to a rdk defined WorldState.
func WorldStateFromProtobuf(proto *commonpb.WorldState) (*WorldState, error) {
	convertProtoGeometries := func(allProtoGeometries []*commonpb.GeometriesInFrame) ([]*GeometriesInFrame, error) {
		list := make([]*GeometriesInFrame, 0, len(allProtoGeometries))
		for _, protoGeometries := range allProtoGeometries {
			geometries, err := ProtobufToGeometriesInFrame(protoGeometries)
			if err != nil {
				return nil, err
			}
			list = append(list, geometries)
		}
		return list, nil
	}
	if proto == nil {
		return &WorldState{}, nil
	}
	obstacles, err := convertProtoGeometries(proto.GetObstacles())
	if err != nil {
		return nil, err
	}
	transforms, err := LinkInFramesFromTransformsProtobuf(proto.GetTransforms())
	if err != nil {
		return nil, err
	}

	return &WorldState{
		Obstacles:  obstacles,
		Transforms: transforms,
	}, nil
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

	transforms, err := LinkInFramesToTransformsProtobuf(worldState.Transforms)
	if err != nil {
		return nil, err
	}

	return &commonpb.WorldState{
		Obstacles:  convertGeometriesToProto(worldState.Obstacles),
		Transforms: transforms,
	}, nil
}

// ObstaclesInWorldFrame takes a frame system and a set of inputs for that frame system and converts all the obstacles
// in the WorldState such that they are in the frame system's World reference frame.
func (ws *WorldState) ObstaclesInWorldFrame(fs FrameSystem, inputs map[string][]Input) (*GeometriesInFrame, error) {
	transformGeometriesToWorldFrame := func(gfs []*GeometriesInFrame) (*GeometriesInFrame, error) {
		nameCheck := make(map[string]bool)
		allGeometries := make([]spatial.Geometry, 0, len(gfs))

		unnamedCount := 1

		for _, gf := range gfs {
			tf, err := fs.Transform(inputs, gf, World)
			if err != nil {
				return nil, err
			}
			for _, g := range tf.(*GeometriesInFrame).Geometries() {
				geomName := g.Label()
				if geomName == "" {
					geomName = unnamedWorldStateGeometryPrefix + strconv.Itoa(unnamedCount)
					g.SetLabel(geomName)
					unnamedCount++
				}

				if _, present := nameCheck[geomName]; present {
					return nil, fmt.Errorf("cannot specify multiple geometries with the same name: %s", geomName)
				}
				nameCheck[geomName] = true
				allGeometries = append(allGeometries, g)
			}
		}
		return NewGeometriesInFrame(World, allGeometries), nil
	}

	if ws == nil {
		ws = &WorldState{}
	}
	return transformGeometriesToWorldFrame(ws.Obstacles)
}
