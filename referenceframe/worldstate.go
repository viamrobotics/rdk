package referenceframe

import (
	commonpb "go.viam.com/api/common/v1"
)

type WorldState struct {
	Obstacles, InteractionSpaces []*GeometriesInFrame
	Transforms                   []*commonpb.Transform
}

func WorldStateFromProtobuf(proto *commonpb.WorldState) (*WorldState, error) {
	convertProtoGeometries := func(allProtoGeometries []*commonpb.GeometriesInFrame) ([]*GeometriesInFrame, error) {
		list := make([]*GeometriesInFrame, 0)
		for _, protoGeometries := range allProtoGeometries {
			geometries, err := GeometriesInFrameFromProtobuf(protoGeometries)
			if err != nil {
				return nil, err
			}
			list = append(list, geometries)
		}
		return list, nil
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

func WorldStateToProtobuf(worldState WorldState) *commonpb.WorldState {
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
