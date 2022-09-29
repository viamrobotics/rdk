package datasync

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"io"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	// Send metadata upload request.
	md, err := f.ReadMetadata()
	if err != nil {
		return err
	}
	sensorData, err := getAllSensorData(f)
	if err != nil {
		return err
	}

	ur := &v1.UnaryUploadRequest{
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
	resp, err := client.UnaryUpload(ctx, ur)
	if err != nil {
		fmt.Println("received error from server")
		fmt.Println(err)
		return err
	}
	fmt.Println("did not receive error from server")
	fmt.Println(err)
	// TODO: real code handling
	if resp.GetCode() != 200 {
		return errors.New("error making upload request: " + resp.GetMessage())
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
