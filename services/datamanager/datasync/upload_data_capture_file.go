package datasync

import (
	"context"

	"github.com/docker/go-units"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/camera/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

// MaxUnaryFileSize is the max number of bytes to send using the unary DataCaptureUpload, as opposed to the
// StreamingDataCaptureUpload.
var MaxUnaryFileSize = int64(units.MB)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	md := f.ReadMetadata()
	sensorData, err := datacapture.SensorDataFromFile(f)
	if err != nil {
		return errors.Wrap(err, "error reading sensor data from file")
	}

	// Do not attempt to upload a file without any sensor readings.
	if len(sensorData) == 0 {
		return nil
	}

	if md.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && len(sensorData) > 1 {
		return errors.New("binary sensor data file with more than one sensor reading is not supported")
	}

	if md.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && md.GetMethodName() == datacapture.GetImages {
		var res pb.GetImagesResponse
		if err := mapstructure.Decode(sensorData[0].GetStruct().AsMap(), &res); err != nil {
			return err
		}

		// If the GetImagesResponse metadata contains a capture timestamp, use that to
		// populate SensorMetadata. Otherwise, use the timestamps that the data management
		// system stored to track when a request was sent and response was received.
		var timeRequested, timeReceived *timestamppb.Timestamp
		timeCaptured := res.GetResponseMetadata().GetCapturedAt()
		if timeCaptured != nil {
			timeRequested, timeReceived = timeCaptured, timeCaptured
		} else {
			sensorMD := sensorData[0].GetMetadata()
			timeRequested = sensorMD.GetTimeRequested()
			timeReceived = sensorMD.GetTimeReceived()
		}

		for _, img := range res.Images {
			newSensorData := []*v1.SensorData{
				{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Binary{
						Binary: img.GetImage(),
					},
				},
			}
			newUploadMD := &v1.UploadMetadata{
				PartId:           partID,
				ComponentType:    md.GetComponentType(),
				ComponentName:    md.GetComponentName(),
				MethodName:       md.GetMethodName(),
				Type:             md.GetType(),
				MethodParameters: md.GetMethodParameters(),
				FileExtension:    getFileExtFromImageFormat(img.GetFormat()),
				Tags:             md.GetTags(),
			}
			if err := uploadSensorData(ctx, client, newUploadMD, newSensorData, f.Size()); err != nil {
				return err
			}
		}
	} else {
		// Build UploadMetadata
		uploadMD := &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    md.GetComponentType(),
			ComponentName:    md.GetComponentName(),
			MethodName:       md.GetMethodName(),
			Type:             md.GetType(),
			MethodParameters: md.GetMethodParameters(),
			FileExtension:    md.GetFileExtension(),
			Tags:             md.GetTags(),
		}
		return uploadSensorData(ctx, client, uploadMD, sensorData, f.Size())
	}
	return nil
}

func uploadSensorData(ctx context.Context, client v1.DataSyncServiceClient, uploadMD *v1.UploadMetadata,
	sensorData []*v1.SensorData, fileSize int64,
) error {
	// If it's a large binary file, we need to upload it in chunks.
	if uploadMD.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && fileSize > MaxUnaryFileSize {
		c, err := client.StreamingDataCaptureUpload(ctx)
		if err != nil {
			return errors.Wrap(err, "error creating upload client")
		}

		toUpload := sensorData[0]

		// First send metadata.
		streamMD := &v1.StreamingDataCaptureUploadRequest_Metadata{
			Metadata: &v1.DataCaptureUploadMetadata{
				UploadMetadata: uploadMD,
				SensorMetadata: toUpload.GetMetadata(),
			},
		}
		if err := c.Send(&v1.StreamingDataCaptureUploadRequest{UploadPacket: streamMD}); err != nil {
			return err
		}

		// Then call the function to send the rest.
		if err := sendStreamingDCRequests(ctx, c, toUpload.GetBinary()); err != nil {
			return errors.Wrap(err, "error sending streaming data capture requests")
		}

		if _, err := c.CloseAndRecv(); err != nil {
			return errors.Wrap(err, "error receiving upload response")
		}
	} else {
		ur := &v1.DataCaptureUploadRequest{
			Metadata:       uploadMD,
			SensorContents: sensorData,
		}
		if _, err := client.DataCaptureUpload(ctx, ur); err != nil {
			return err
		}
	}

	return nil
}

func sendStreamingDCRequests(ctx context.Context, stream v1.DataSyncService_StreamingDataCaptureUploadClient,
	contents []byte,
) error {
	// Loop until there is no more content to send.
	for i := 0; i < len(contents); i += UploadChunkSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Get the next chunk from contents.
			end := i + UploadChunkSize
			if end > len(contents) {
				end = len(contents)
			}
			chunk := contents[i:end]

			// Build request with contents.
			uploadReq := &v1.StreamingDataCaptureUploadRequest{
				UploadPacket: &v1.StreamingDataCaptureUploadRequest_Data{
					Data: chunk,
				},
			}

			// Send request
			if err := stream.Send(uploadReq); err != nil {
				return err
			}
		}
	}

	return nil
}

func getFileExtFromImageFormat(res pb.Format) string {
	switch res {
	case pb.Format_FORMAT_JPEG:
		return ".jpeg"
	case pb.Format_FORMAT_PNG:
		return ".png"
	case pb.Format_FORMAT_RAW_DEPTH:
		return ".dep"
	case pb.Format_FORMAT_RAW_RGBA:
		return ".rgba"
	case pb.Format_FORMAT_UNSPECIFIED:
		fallthrough
	default:
		return ""
	}
}
