package datasync

import (
	"context"

	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	md := f.ReadMetadata()
	sensorData, err := datacapture.SensorDataFromFile(f)
	if err != nil {
		return err
	}

	// Do not attempt to upload a file without any sensor readings.
	if len(sensorData) == 0 {
		return nil
	}

	ur := &v1.DataCaptureUploadRequest{
		Metadata: &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    md.GetComponentType(),
			ComponentName:    md.GetComponentName(),
			ComponentModel:   md.GetComponentModel(),
			MethodName:       md.GetMethodName(),
			Type:             md.GetType(),
			MethodParameters: md.GetMethodParameters(),
			FileExtension:    md.GetFileExtension(),
			Tags:             md.GetTags(),
			SessionId:        md.GetSessionId(),
		},
		SensorContents: sensorData,
	}
	_, err = client.DataCaptureUpload(ctx, ur)
	if err != nil {
		return err
	}

	return nil
}
