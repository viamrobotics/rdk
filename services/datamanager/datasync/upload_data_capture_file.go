package datasync

import (
	"context"
	"io"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	md := f.ReadMetadata()
	f.Reset()
	sensorData, err := getAllSensorData(f)
	if err != nil {
		return err
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

func getAllSensorData(f *datacapture.File) ([]*v1.SensorData, error) {
	var ret []*v1.SensorData
	for {
		next, err := f.ReadNext()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		ret = append(ret, next)
	}
	return ret, nil
}
