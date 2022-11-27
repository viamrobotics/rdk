package referenceframe

import (
	"strconv"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	spatial "go.viam.com/rdk/spatialmath"
)

// WorldState is a struct to store the data representation of the robot's environment.
type WorldState struct {
	Obstacles, InteractionSpaces []*GeometriesInFrame
	Transforms                   []*commonpb.Transform
}

// WorldStateFromProtobuf takes the protobuf definition of a WorldState and converts it to a rdk defined WorldState.
func WorldStateFromProtobuf(proto *commonpb.WorldState) (*WorldState, error) {
	convertProtoGeometries := func(allProtoGeometries []*commonpb.GeometriesInFrame) ([]*GeometriesInFrame, error) {
		list := make([]*GeometriesInFrame, 0)
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
	obstacles, err := convertProtoGeometries(proto.Obstacles)
	if err != nil {
		return nil, err
	}
	interactionSpaces, err := convertProtoGeometries(proto.InteractionSpaces)
	if err != nil {
		return nil, err
	}

	return &WorldState{
		Obstacles:         obstacles,
		InteractionSpaces: interactionSpaces,
		Transforms:        proto.GetTransforms(),
	}, nil
}

// WorldStateToProtobuf takes an rdk WorldState and converts it to the protobuf definition of a WorldState.
func WorldStateToProtobuf(worldState *WorldState) *commonpb.WorldState {
	convertGeometriesToProto := func(allGeometries []*GeometriesInFrame) []*commonpb.GeometriesInFrame {
		list := make([]*commonpb.GeometriesInFrame, 0)
		for _, geometries := range allGeometries {
			list = append(list, GeometriesInFrameToProtobuf(geometries))
		}
		return list
	}

	return &commonpb.WorldState{
		Obstacles:         convertGeometriesToProto(worldState.Obstacles),
		InteractionSpaces: convertGeometriesToProto(worldState.InteractionSpaces),
		Transforms:        worldState.Transforms,
	}
}

// ToWorldFrame takes a frame system and a set of inputs for that frame system and converts all the geometries
// in the WorldState such that they are in the frame system's World reference frame.
func (ws *WorldState) ToWorldFrame(fs FrameSystem, inputs map[string][]Input) (*WorldState, error) {
	transformGeometriesToWorldFrame := func(gfs []*GeometriesInFrame) (*GeometriesInFrame, error) {
		allGeometries := make(map[string]spatial.Geometry)
		for name1, gf := range gfs {
			tf, err := fs.Transform(inputs, gf, World)
			if err != nil {
				return nil, err
			}
			for name2, g := range tf.(*GeometriesInFrame).Geometries() {
				geomName := strconv.Itoa(name1) + "_" + name2
				if _, present := allGeometries[geomName]; present {
					return nil, errors.New("multiple geometries with the same name")
				}
				allGeometries[geomName] = g
			}
		}
		return NewGeometriesInFrame(World, allGeometries), nil
	}

	if ws == nil {
		ws = &WorldState{}
	}
	obstacles, err := transformGeometriesToWorldFrame(ws.Obstacles)
	if err != nil {
		return nil, err
	}
	interactionSpaces, err := transformGeometriesToWorldFrame(ws.InteractionSpaces)
	if err != nil {
		return nil, err
	}
	return &WorldState{
		Obstacles:         []*GeometriesInFrame{obstacles},
		InteractionSpaces: []*GeometriesInFrame{interactionSpaces},
	}, nil
}
