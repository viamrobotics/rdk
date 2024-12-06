package sync

import (
	"context"

	"github.com/docker/go-units"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	datasyncPB "go.viam.com/api/app/datasync/v1"
	cameraPB "go.viam.com/api/component/camera/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
)

var (
	// MaxUnaryFileSize is the max number of bytes to send using the unary DataCaptureUpload, as opposed to the
	// StreamingDataCaptureUpload.
	MaxUnaryFileSize                          = int64(units.MB)
	errMultipleReadingTypes                   = errors.New("sensor readings contain multiple types")
	errSensorDataTypesDontMatchUploadMetadata = errors.New("sensor readings types don't match upload metadata")
	errInvalidCaptureFileType                 = errors.New("invalid capture file type")
	// terminalCaptureFileErrs is the set of errors that will result in exponential retries stoping to retry
	// uploading a data capture file and instead move it to a failed directory.
	terminalCaptureFileErrs = []error{
		errMultipleReadingTypes,
		errSensorDataTypesDontMatchUploadMetadata,
		errInvalidCaptureFileType,
	}
)

// uploadDataCaptureFile uploads the *data.CaptureFile to the cloud using the cloud connection.
// returns context.Cancelled if ctx is cancelled before upload completes.
// If f is of type BINARY_SENSOR and its size is over MaxUnaryFileSize,
// uses StreamingDataCaptureUpload API so as to not exceed the unary response size.
// Otherwise, uploads data over DataCaptureUpload API.
// Note: the bytes size returned is the size of the input file. It only returns a non 0 value in the success case.
func uploadDataCaptureFile(ctx context.Context, f *data.CaptureFile, conn cloudConn, logger logging.Logger) (uint64, error) {
	logger.Debugf("preparing to upload data capture file: %s, size: %d", f.GetPath(), f.Size())

	md := f.ReadMetadata()
	sensorData, err := data.SensorDataFromCaptureFile(f)
	if err != nil {
		return 0, errors.Wrap(err, "error reading sensor data")
	}

	// Do not attempt to upload a file without any sensor readings.
	if len(sensorData) == 0 {
		logger.Warnf("ignoring and deleting empty capture file without syncing it: %s", f.GetPath())
		// log here as this will delete a .capture file without uploading it and without moving it to the failed directory
		return 0, nil
	}

	sensorDataTypeSet := captureTypesInSensorData(sensorData)
	if len(sensorDataTypeSet) != 1 {
		return 0, errMultipleReadingTypes
	}

	_, isTabular := sensorDataTypeSet[data.CaptureTypeTabular]
	if isLegacyGetImagesCaptureFile(md, isTabular) {
		logger.Debugf("attemping to upload legacy camera.GetImages data: %s", f.GetPath())
		return uint64(f.Size()), legacyUploadGetImages(ctx, conn, md, sensorData[0], f.Size(), f.GetPath(), logger)
	}

	if err := checkUploadMetadaTypeMatchesSensorDataType(md, sensorDataTypeSet); err != nil {
		return 0, err
	}

	metaData := uploadMetadata(conn.partID, md)
	return uint64(f.Size()), uploadSensorData(ctx, conn.client, metaData, sensorData, f.Size(), f.GetPath(), logger)
}

func checkUploadMetadaTypeMatchesSensorDataType(md *datasyncPB.DataCaptureMetadata, sensorDataTypeSet map[data.CaptureType]struct{}) error {
	var captureType data.CaptureType
	// get first element of single element set
	for k := range sensorDataTypeSet {
		captureType = k
		break
	}

	if captureType.ToProto() != md.GetType() {
		return errSensorDataTypesDontMatchUploadMetadata
	}
	return nil
}

func isLegacyGetImagesCaptureFile(md *datasyncPB.DataCaptureMetadata, isTabular bool) bool {
	// camera.GetImages is a special case. For that API we make 2 binary data upload requests
	// if the capture file proports to contain DataType_DATA_TYPE_BINARY_SENSOR from GetImages
	// but the sensor data is not binary, then this is from a legacy GetImages capture file
	return md.GetType() == datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR && md.GetMethodName() == data.GetImages && isTabular
}

func captureTypesInSensorData(sensorData []*datasyncPB.SensorData) map[data.CaptureType]struct{} {
	set := map[data.CaptureType]struct{}{}
	for _, sd := range sensorData {
		if data.IsBinary(sd) {
			if _, ok := set[data.CaptureTypeBinary]; !ok {
				set[data.CaptureTypeBinary] = struct{}{}
			}
		} else {
			if _, ok := set[data.CaptureTypeTabular]; !ok {
				set[data.CaptureTypeTabular] = struct{}{}
			}
		}
	}
	return set
}

func uploadMetadata(partID string, md *datasyncPB.DataCaptureMetadata) *datasyncPB.UploadMetadata {
	return &datasyncPB.UploadMetadata{
		PartId:           partID,
		ComponentType:    md.GetComponentType(),
		ComponentName:    md.GetComponentName(),
		MethodName:       md.GetMethodName(),
		Type:             md.GetType(),
		MethodParameters: md.GetMethodParameters(),
		Tags:             md.GetTags(),
		FileExtension:    md.GetFileExtension(),
	}
}

func legacyUploadGetImages(
	ctx context.Context,
	conn cloudConn,
	md *datasyncPB.DataCaptureMetadata,
	sd *datasyncPB.SensorData,
	size int64,
	path string,
	logger logging.Logger,
) error {
	var res cameraPB.GetImagesResponse
	if err := mapstructure.Decode(sd.GetStruct().AsMap(), &res); err != nil {
		return errors.Wrap(err, "failed to decode camera.GetImagesResponse")
	}
	timeRequested, timeReceived := getImagesTimestamps(&res, sd)

	for i, img := range res.Images {
		newSensorData := []*datasyncPB.SensorData{
			{
				Metadata: &datasyncPB.SensorMetadata{
					TimeRequested: timeRequested,
					TimeReceived:  timeReceived,
				},
				Data: &datasyncPB.SensorData_Binary{
					Binary: img.GetImage(),
				},
			},
		}
		logger.Debugf("attempting to upload camera.GetImages response, index: %d", i)
		metadata := uploadMetadata(conn.partID, md)
		metadata.FileExtension = getFileExtFromImageFormat(img.GetFormat())
		// TODO: This is wrong as the size describes the size of the entire GetImages response, but we are only
		// uploading one of the 2 images in that response here.
		if err := uploadSensorData(ctx, conn.client, metadata, newSensorData, size, path, logger); err != nil {
			return errors.Wrapf(err, "failed uploading GetImages image index: %d", i)
		}
	}
	return nil
}

func getImagesTimestamps(
	res *cameraPB.GetImagesResponse,
	sensorData *datasyncPB.SensorData,
) (*timestamppb.Timestamp, *timestamppb.Timestamp) {
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

func uploadSensorData(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	uploadMD *datasyncPB.UploadMetadata,
	sensorData []*datasyncPB.SensorData,
	fileSize int64,
	path string,
	logger logging.Logger,
) error {
	captureFileType := uploadMD.GetType()
	switch captureFileType {
	case datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR:
		// If it's a large binary file, we need to upload it in chunks.
		if uploadMD.GetType() == datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR && fileSize > MaxUnaryFileSize {
			return uploadMultipleLargeBinarySensorData(ctx, client, uploadMD, sensorData, path, logger)
		}
		return uploadMultipleBinarySensorData(ctx, client, uploadMD, sensorData, path, logger)
	case datasyncPB.DataType_DATA_TYPE_TABULAR_SENSOR:
		// Otherwise use the unary endpoint
		logger.Debugf("attempting to upload small binary file using DataCaptureUpload, file: %s", path)
		_, err := client.DataCaptureUpload(ctx, &datasyncPB.DataCaptureUploadRequest{
			Metadata:       uploadMD,
			SensorContents: sensorData,
		})
		return errors.Wrap(err, "DataCaptureUpload failed")
	case datasyncPB.DataType_DATA_TYPE_FILE:
		fallthrough
	case datasyncPB.DataType_DATA_TYPE_UNSPECIFIED:
		fallthrough
	default:
		logger.Errorf("%s: %s", errInvalidCaptureFileType.Error(), captureFileType)
		return errInvalidCaptureFileType
	}
}

func uploadBinarySensorData(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	md *datasyncPB.UploadMetadata,
	sd *datasyncPB.SensorData,
) error {
	// if the binary sensor data has a mime type, set the file extension
	// to match
	fileExtensionFromMimeType := getFileExtFromMimeType(sd.GetMetadata().GetMimeType())
	if fileExtensionFromMimeType != "" {
		md.FileExtension = fileExtensionFromMimeType
	}
	if _, err := client.DataCaptureUpload(ctx, &datasyncPB.DataCaptureUploadRequest{
		Metadata:       md,
		SensorContents: []*datasyncPB.SensorData{sd},
	}); err != nil {
		return errors.Wrap(err, "DataCaptureUpload failed")
	}

	return nil
}

func uploadMultipleBinarySensorData(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	uploadMD *datasyncPB.UploadMetadata,
	sensorData []*datasyncPB.SensorData,
	path string,
	logger logging.Logger,
) error {
	// this is the common case
	if len(sensorData) == 1 {
		logger.Debugf("attempting to upload small binary file using DataCaptureUpload, sensor data, file: %s", path)
		return uploadBinarySensorData(ctx, client, uploadMD, sensorData[0])
	}

	// we only go down this path if the capture method returned multiple binary
	// responses, which at time of writing, only includes camera.GetImages data.
	for i, sd := range sensorData {
		logger.Debugf("attempting to upload small binary file using DataCaptureUpload, sensor data index: %d, ext: %s, file: %s", i, path)
		// we clone as the uploadMD may be changed for each sensor data
		// and I'm not confident that it is safe to reuse grpc request structs
		// between calls if the data in the request struct changes
		clonedMD := proto.Clone(uploadMD).(*datasyncPB.UploadMetadata)
		if err := uploadBinarySensorData(ctx, client, clonedMD, sd); err != nil {
			return err
		}
	}
	return nil
}

func uploadMultipleLargeBinarySensorData(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	uploadMD *datasyncPB.UploadMetadata,
	sensorData []*datasyncPB.SensorData,
	path string,
	logger logging.Logger,
) error {
	if len(sensorData) == 1 {
		logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, sensor data file: %s", path)
		return uploadLargeBinarySensorData(ctx, client, uploadMD, sensorData[0], path, logger)
	}

	for i, sd := range sensorData {
		logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, sensor data index: %d, file: %s", i, path)
		// we clone as the uploadMD may be changed for each sensor data
		// and I'm not confident that it is safe to reuse grpc request structs
		// between calls if the data in the request struct changes
		clonedMD := proto.Clone(uploadMD).(*datasyncPB.UploadMetadata)
		if err := uploadLargeBinarySensorData(ctx, client, clonedMD, sd, path, logger); err != nil {
			return err
		}
	}
	return nil
}

func uploadLargeBinarySensorData(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	md *datasyncPB.UploadMetadata,
	sd *datasyncPB.SensorData,
	path string,
	logger logging.Logger,
) error {
	c, err := client.StreamingDataCaptureUpload(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating StreamingDataCaptureUpload client")
	}
	// if the binary sensor data has a mime type, set the file extension
	// to match
	smd := sd.GetMetadata()
	fileExtensionFromMimeType := getFileExtFromMimeType(smd.GetMimeType())
	if fileExtensionFromMimeType != "" {
		md.FileExtension = fileExtensionFromMimeType
	}
	streamMD := &datasyncPB.StreamingDataCaptureUploadRequest_Metadata{
		Metadata: &datasyncPB.DataCaptureUploadMetadata{
			UploadMetadata: md,
			SensorMetadata: smd,
		},
	}
	if err := c.Send(&datasyncPB.StreamingDataCaptureUploadRequest{UploadPacket: streamMD}); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed sending metadata")
	}

	// Then call the function to send the rest.
	if err := sendStreamingDCRequests(ctx, c, sd.GetBinary(), path, logger); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed to sync")
	}

	if _, err = c.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload CloseAndRecv failed")
	}

	return nil
}

func sendStreamingDCRequests(
	ctx context.Context,
	stream datasyncPB.DataSyncService_StreamingDataCaptureUploadClient,
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
			uploadReq := &datasyncPB.StreamingDataCaptureUploadRequest{
				UploadPacket: &datasyncPB.StreamingDataCaptureUploadRequest_Data{
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

func getFileExtFromImageFormat(t cameraPB.Format) string {
	switch t {
	case cameraPB.Format_FORMAT_JPEG:
		return data.ExtJpeg
	case cameraPB.Format_FORMAT_PNG:
		return data.ExtPng
	case cameraPB.Format_FORMAT_RAW_DEPTH:
		return ".dep"
	case cameraPB.Format_FORMAT_RAW_RGBA:
		return ".rgba"
	case cameraPB.Format_FORMAT_UNSPECIFIED:
		fallthrough
	default:
		return data.ExtDefault
	}
}

func getFileExtFromMimeType(t datasyncPB.MimeType) string {
	switch t {
	case datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG:
		return data.ExtJpeg
	case datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG:
		return data.ExtPng
	case datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD:
		return data.ExtPcd
	case datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return data.ExtDefault
	}
}
