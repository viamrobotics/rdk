package sync

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/go-units"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/camera/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
)

// MaxUnaryFileSize is the max number of bytes to send using the unary DataCaptureUpload, as opposed to the
// StreamingDataCaptureUpload.
var MaxUnaryFileSize = int64(units.MB)

// uploadDataCaptureFile uploads the *data.CaptureFile to the cloud using the cloud connection.
// returns context.Cancelled if ctx is cancelled before upload completes.
// If f is of type BINARY_SENSOR and its size is over MaxUnaryFileSize,
// uses StreamingDataCaptureUpload API so as to not exceed the unary response size.
// Otherwise, uploads data over DataCaptureUpload API.
// Note: the bytes size returned is the size of the input file. It only returns a non 0 value in the success case.
func uploadDataCaptureFile(ctx context.Context, f *data.CaptureFile, conn cloudConn, flag bool, logger logging.Logger) (uint64, error) {
	logger.Debugf("preparing to upload data capture file: %s, size: %d", f.GetPath(), f.Size())

	md := f.ReadMetadata()

	// camera.GetImages is a special case. For that API we make 2 binary data upload requests
	if md.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && md.GetMethodName() == data.GetImages {
		return uint64(f.Size()), uploadGetImages(ctx, conn, md, f, logger)
	}

	metaData := uploadMetadata(conn.partID, md, md.GetFileExtension())
	if md.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && flag {
		return uint64(f.Size()), uploadChunkedBinaryData(ctx, conn.client, metaData, f, logger)
	}

	sensorData, err := data.SensorDataFromCaptureFile(f)
	if err != nil {
		return 0, errors.Wrap(err, "error reading sensor data")
	}

	if len(sensorData) == 0 {
		logger.Warnf("ignoring and deleting empty capture file without syncing it: %s", f.GetPath())
		// log here as this will delete a .capture file without uploading it and without moving it to the failed directory
		return 0, nil
	}

	return uint64(f.Size()), uploadSensorData(ctx, conn.client, metaData, sensorData, f.Size(), f.GetPath(), logger)
}

func uploadMetadata(partID string, md *v1.DataCaptureMetadata, fileextension string) *v1.UploadMetadata {
	return &v1.UploadMetadata{
		PartId:           partID,
		ComponentType:    md.GetComponentType(),
		ComponentName:    md.GetComponentName(),
		MethodName:       md.GetMethodName(),
		Type:             md.GetType(),
		MethodParameters: md.GetMethodParameters(),
		Tags:             md.GetTags(),
		FileExtension:    fileextension,
	}
}

func uploadGetImages(
	ctx context.Context,
	conn cloudConn,
	md *v1.DataCaptureMetadata,
	f *data.CaptureFile,
	logger logging.Logger,
) error {
	logger.Debugf("attemping to upload camera.GetImages data: %s", f.GetPath())

	sensorData, err := data.SensorDataFromCaptureFile(f)
	if err != nil {
		return errors.Wrap(err, "error reading sensor data")
	}

	if len(sensorData) == 0 {
		logger.Warnf("ignoring and deleting empty capture file without syncing it: %s", f.GetPath())
		// log here as this will delete a .capture file without uploading it and without moving it to the failed directory
		return nil
	}

	if len(sensorData) > 1 {
		return fmt.Errorf("binary sensor data file with more than one sensor reading is not supported: %s", f.GetPath())
	}
	sd := sensorData[0]
	var res pb.GetImagesResponse
	if err := mapstructure.Decode(sd.GetStruct().AsMap(), &res); err != nil {
		return errors.Wrap(err, "failed to decode camera.GetImagesResponse")
	}
	timeRequested, timeReceived := getImagesTimestamps(&res, sd)

	for i, img := range res.Images {
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
		logger.Debugf("attempting to upload camera.GetImages response, index: %d", i)
		metadata := uploadMetadata(conn.partID, md, getFileExtFromImageFormat(img.GetFormat()))
		// TODO: This is wrong as the size describes the size of the entire GetImages response, but we are only
		// uploading one of the 2 images in that response here.
		if err := uploadSensorData(ctx, conn.client, metadata, newSensorData, f.Size(), f.GetPath(), logger); err != nil {
			return errors.Wrapf(err, "failed uploading GetImages image index: %d", i)
		}
	}
	return nil
}

func getImagesTimestamps(res *pb.GetImagesResponse, sensorData *v1.SensorData) (*timestamppb.Timestamp, *timestamppb.Timestamp) {
	// If the GetImagesResponse metadata contains a capture timestamp, use that to
	// populate SensorMetadata. Otherwise, use the timestamps that the data management
	// system stored to track when a request was sent and response was received.
	var timeRequested, timeReceived *timestamppb.Timestamp
	timeCaptured := res.GetResponseMetadata().GetCapturedAt()
	if timeCaptured != nil {
		timeRequested, timeReceived = timeCaptured, timeCaptured
	} else {
		sensorMD := sensorData.GetMetadata()
		timeRequested = sensorMD.GetTimeRequested()
		timeReceived = sensorMD.GetTimeReceived()
	}
	return timeRequested, timeReceived
}

func uploadChunkedBinaryData(
	ctx context.Context,
	client v1.DataSyncServiceClient,
	uploadMD *v1.UploadMetadata,
	f *data.CaptureFile,
	logger logging.Logger,
) error {
	// If it's a large binary file, we need to upload it in chunks.
	logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, file: %s", f.GetPath())
	var smd v1.SensorMetadata
	r, err := f.BinaryReader(&smd)
	if err != nil {
		return err
	}
	c, err := client.StreamingDataCaptureUpload(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating StreamingDataCaptureUpload client")
	}

	// First send metadata.
	streamMD := &v1.StreamingDataCaptureUploadRequest_Metadata{
		Metadata: &v1.DataCaptureUploadMetadata{
			UploadMetadata: uploadMD,
			SensorMetadata: &smd,
		},
	}
	if err := c.Send(&v1.StreamingDataCaptureUploadRequest{UploadPacket: streamMD}); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed sending metadata")
	}

	// Then call the function to send the rest.
	if err := sendChunkedStreamingDCRequests(ctx, c, r, f.GetPath(), logger); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed to sync")
	}

	_, err = c.CloseAndRecv()
	return errors.Wrap(err, "StreamingDataCaptureUpload CloseAndRecv failed")
}

func uploadSensorData(
	ctx context.Context,
	client v1.DataSyncServiceClient,
	uploadMD *v1.UploadMetadata,
	sensorData []*v1.SensorData,
	fileSize int64,
	path string,
	logger logging.Logger,
) error {
	// If it's a large binary file, we need to upload it in chunks.
	if uploadMD.GetType() == v1.DataType_DATA_TYPE_BINARY_SENSOR && fileSize > MaxUnaryFileSize {
		logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, file: %s", path)
		c, err := client.StreamingDataCaptureUpload(ctx)
		if err != nil {
			return errors.Wrap(err, "error creating StreamingDataCaptureUpload client")
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
			return errors.Wrap(err, "StreamingDataCaptureUpload failed sending metadata")
		}

		// Then call the function to send the rest.
		if err := sendStreamingDCRequests(ctx, c, toUpload.GetBinary(), path, logger); err != nil {
			return errors.Wrap(err, "StreamingDataCaptureUpload failed to sync")
		}

		_, err = c.CloseAndRecv()
		return errors.Wrap(err, "StreamingDataCaptureUpload CloseAndRecv failed")
	}

	// Otherwise use the unary endpoint
	logger.Debugf("attempting to upload small binary file using DataCaptureUpload, file: %s", path)
	_, err := client.DataCaptureUpload(ctx, &v1.DataCaptureUploadRequest{
		Metadata:       uploadMD,
		SensorContents: sensorData,
	})
	return errors.Wrap(err, "DataCaptureUpload failed")
}

func sendChunkedStreamingDCRequests(
	ctx context.Context,
	stream v1.DataSyncService_StreamingDataCaptureUploadClient,
	r io.Reader,
	path string,
	logger logging.Logger,
) error {
	chunk := make([]byte, UploadChunkSize)
	// Loop until there is no more content to send.
	chunkCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, errRead := r.Read(chunk)
			if n > 0 {
				// if there is data, send it
				// Build request with contents.
				uploadReq := &v1.StreamingDataCaptureUploadRequest{
					UploadPacket: &v1.StreamingDataCaptureUploadRequest_Data{
						Data: chunk[:n],
					},
				}

				// Send request
				logger.Debugf("datasync.StreamingDataCaptureUpload sending chunk %d for file: %s", chunkCount, path)
				if errSend := stream.Send(uploadReq); errSend != nil {
					return errSend
				}
			}

			// if we reached the end of the file return nil err (success)
			if errors.Is(errRead, io.EOF) {
				return nil
			}

			// if Read hit an unexpected error, return the error
			if errRead != nil {
				return errRead
			}
			chunkCount++
		}
	}
}

func sendStreamingDCRequests(
	ctx context.Context,
	stream v1.DataSyncService_StreamingDataCaptureUploadClient,
	contents []byte,
	path string,
	logger logging.Logger,
) error {
	// Loop until there is no more content to send.
	chunkCount := 0
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
			logger.Debugf("datasync.StreamingDataCaptureUpload sending chunk %d starting at byte index %d for file: %s", chunkCount, i, path)
			if err := stream.Send(uploadReq); err != nil {
				return err
			}
			chunkCount++
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
