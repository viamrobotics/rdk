package datasync

import (
	"context"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"os"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	// Send metadata upload request.
	md, err := f.ReadMetadata()
	if err != nil {
		return err
	}
	_ = &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:           partID,
				ComponentType:    md.GetComponentType(),
				ComponentName:    md.GetComponentName(),
				ComponentModel:   md.GetComponentModel(),
				MethodName:       md.GetMethodName(),
				Type:             md.GetType(),
				MethodParameters: md.GetMethodParameters(),
				Tags:             md.GetTags(),
				SessionId:        md.GetSessionId(),
			},
		},
	}

	// TODO: make unary upload call with whole contents of file. return error if receive error.
	return nil
}

func getNextSensorUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
		// Get the next sensor data reading from file, check for an error.
		next, err := datacapture.ReadNextSensorData(f)
		if err != nil {
			return nil, err
		}
		// Otherwise, return an UploadRequest and no error.
		return &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_SensorContents{
				SensorContents: next,
			},
		}, nil
	}
}
