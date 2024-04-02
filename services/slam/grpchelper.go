package slam

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/slam/v1"
)

// PointCloudMapCallback helps a client request the point cloud stream from a SLAM server,
// returning a callback function for accessing the stream data.
func PointCloudMapCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient, returnEditedMap bool) (
	func() ([]byte, error), error,
) {
	req := &pb.GetPointCloudMapRequest{Name: name, ReturnEditedMap: &returnEditedMap}

	// If the target gRPC server returns an error status, this call doesn't return an error.
	// Instead, the error status will be returned to the first call to resp.Recv().
	// This call only returns an error if the connection to the target gRPC server can't be established, is canceled, etc.
	resp, err := slamClient.GetPointCloudMap(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting the pointcloud map from the SLAM client")
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, errors.Wrap(err, "error receiving pointcloud chunk")
		}

		return chunk.GetPointCloudPcdChunk(), err
	}

	return f, nil
}

// InternalStateCallback helps a client request the internal state stream from a SLAM server,
// returning a callback function for accessing the stream data.
func InternalStateCallback(ctx context.Context, name string, slamClient pb.SLAMServiceClient) (func() ([]byte, error), error) {
	req := &pb.GetInternalStateRequest{Name: name}

	// If the target gRPC server returns an error status, this call doesn't return an error.
	// Instead, the error status will be returned to the first call to resp.Recv().
	// This call only returns an error if the connection to the target gRPC server can't be established, is canceled, etc.
	resp, err := slamClient.GetInternalState(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "error getting the internal state from the SLAM client")
	}

	f := func() ([]byte, error) {
		chunk, err := resp.Recv()
		if err != nil {
			return nil, errors.Wrap(err, "error receiving internal state chunk")
		}

		return chunk.GetInternalStateChunk(), nil
	}
	return f, err
}

// mappingModeToProtobuf converts a MappingMode value to a protobuf MappingMode value.
func mappingModeToProtobuf(mappingMode MappingMode) pb.MappingMode {
	switch mappingMode {
	case MappingModeNewMap:
		return pb.MappingMode_MAPPING_MODE_CREATE_NEW_MAP
	case MappingModeLocalizationOnly:
		return pb.MappingMode_MAPPING_MODE_LOCALIZE_ONLY
	case MappingModeUpdateExistingMap:
		return pb.MappingMode_MAPPING_MODE_UPDATE_EXISTING_MAP
	default:
		return pb.MappingMode_MAPPING_MODE_UNSPECIFIED
	}
}

// protobufToMappingMode converts protobuf MappingMode value to a MappingMode value.
func protobufToMappingMode(mappingMode pb.MappingMode) (MappingMode, error) {
	switch mappingMode {
	case pb.MappingMode_MAPPING_MODE_CREATE_NEW_MAP:
		return MappingModeNewMap, nil
	case pb.MappingMode_MAPPING_MODE_LOCALIZE_ONLY:
		return MappingModeLocalizationOnly, nil
	case pb.MappingMode_MAPPING_MODE_UPDATE_EXISTING_MAP:
		return MappingModeUpdateExistingMap, nil
	case pb.MappingMode_MAPPING_MODE_UNSPECIFIED:
		fallthrough
	default:
		return 0, errors.New("mapping mode unspecified")
	}
}

// sensorTypeToProtobuf converts a SensorType value to a protobuf SensorType value.
func sensorTypeToProtobuf(sensorType SensorType) pb.SensorType {
	switch sensorType {
	case SensorTypeCamera:
		return pb.SensorType_SENSOR_TYPE_CAMERA
	case SensorTypeMovementSensor:
		return pb.SensorType_SENSOR_TYPE_MOVEMENT_SENSOR
	default:
		return pb.SensorType_SENSOR_TYPE_UNSPECIFIED
	}
}

// protobufToSensorType converts protobuf SensorType value to a SensorType value.
func protobufToSensorType(sensorType pb.SensorType) (SensorType, error) {
	switch sensorType {
	case pb.SensorType_SENSOR_TYPE_CAMERA:
		return SensorTypeCamera, nil
	case pb.SensorType_SENSOR_TYPE_MOVEMENT_SENSOR:
		return SensorTypeMovementSensor, nil
	case pb.SensorType_SENSOR_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return 0, errors.New("sensor type unspecified")
	}
}
