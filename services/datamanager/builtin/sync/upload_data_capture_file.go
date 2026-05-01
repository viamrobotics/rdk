package sync

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"

	"github.com/docker/go-units"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	datasyncPB "go.viam.com/api/app/datasync/v1"
	cameraPB "go.viam.com/api/component/camera/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
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
func uploadDataCaptureFile(
	ctx context.Context, f *data.CaptureFile, conn cloudConn, logger logging.Logger, bytesUploadingCounter *atomic.Uint64,
) (uint64, error) {
	logger.Debugf("preparing to upload data capture file: %s, size: %d", f.GetPath(), f.Size())

	md := f.ReadMetadata()

	if md.GetType() == datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR {
		n, err := uploadBinaryPayloads(ctx, f, conn, md, logger, bytesUploadingCounter)
		switch {
		case err == nil:
			return n, nil
		case errors.Is(err, data.ErrNoBinaryField):
			// No binary payload - maybe tabular data in a BINARY_SENSOR-typed file
			// (like legacy camera.GetImages file) - fall through to in-memory path.
		case errors.Is(err, data.ErrSensorMetadataTooLarge) || errors.Is(err, data.ErrUnparsableBinaryCapture):
			// File cannot be streamed due to unexpected format. Fall back with a warning.
			logger.Warnf("cannot stream binary payload for %s, falling back to in-memory upload: %v", f.GetPath(), err)
		default:
			return 0, err
		}
	}

	return uploadFromMemory(ctx, f, conn, md, logger, bytesUploadingCounter)
}

// uploadBinaryPayloads streams each binary payload in f directly from disk without
// loading it into memory. Returns data.ErrNoBinaryField if the first message has no
// binary field, which indicates a legacy camera.GetImages file that must be handled
// by uploadFromMemory instead.
func uploadBinaryPayloads(
	ctx context.Context,
	f *data.CaptureFile,
	conn cloudConn,
	md *datasyncPB.DataCaptureMetadata,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) (uint64, error) {
	f.Reset()
	uploadMD := uploadMetadata(conn.partID, md)
	msgIdx := 0
	for {
		sensorMeta, payloadLen, r, err := f.BinaryPayloadReader()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			if errors.Is(err, data.ErrNoBinaryField) {
				if msgIdx == 0 {
					return 0, err
				}
				// binary capture files currently contain one message, so this should be unreachable
				// but returning a fresh error prevents the outer switch from re-uploading already-streamed messages.
				return 0, errors.New("unexpected shape of binary capture files")
			}
			return 0, errors.Wrap(err, "reading binary payload from capture file")
		}
		clonedMD := proto.Clone(uploadMD).(*datasyncPB.UploadMetadata)
		if payloadLen > MaxUnaryFileSize {
			logger.Debugf("streaming large binary payload (%d bytes), message %d: %s", payloadLen, msgIdx, f.GetPath())
			if err := uploadLargeBinaryFromReader(ctx, conn.client, clonedMD, sensorMeta, r, f.GetPath(),
				logger, bytesUploadingCounter); err != nil {
				return 0, err
			}
		} else {
			logger.Debugf("uploading small binary payload (%d bytes), message %d: %s", payloadLen, msgIdx, f.GetPath())
			payload, err := io.ReadAll(r)
			if err != nil {
				return 0, errors.Wrap(err, "reading binary payload into memory")
			}
			sd := &datasyncPB.SensorData{
				Metadata: sensorMeta,
				Data:     &datasyncPB.SensorData_Binary{Binary: payload},
			}
			if err := uploadBinarySensorData(ctx, conn.client, clonedMD, sd, bytesUploadingCounter); err != nil {
				return 0, err
			}
		}
		msgIdx++
	}
	if msgIdx == 0 {
		return 0, data.ErrNoBinaryField
	}
	return uint64(f.Size()), nil
}

// uploadFromMemory loads all sensor data from f into memory and uploads it. It handles
// tabular files and legacy camera.GetImages binary files.
func uploadFromMemory(
	ctx context.Context,
	f *data.CaptureFile,
	conn cloudConn,
	md *datasyncPB.DataCaptureMetadata,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) (uint64, error) {
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
		return uint64(f.Size()), legacyUploadGetImages(ctx, conn, md, sensorData[0], f.Size(), f.GetPath(), logger, bytesUploadingCounter)
	}

	if err := checkUploadMetadaTypeMatchesSensorDataType(md, sensorDataTypeSet); err != nil {
		return 0, err
	}

	metaData := uploadMetadata(conn.partID, md)
	return uint64(f.Size()), uploadSensorData(ctx, conn.client, metaData, sensorData, f.Size(), f.GetPath(), logger, bytesUploadingCounter)
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
		MimeType:         md.GetMimeType(),
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
	bytesUploadingCounter *atomic.Uint64,
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
		metadata.FileExtension = getFileExtFromImageMimeType(img.GetMimeType())
		// TODO: This is wrong as the size describes the size of the entire GetImages response, but we are only
		// uploading one of the 2 images in that response here.
		if err := uploadSensorData(ctx, conn.client, metadata, newSensorData, size, path, logger, bytesUploadingCounter); err != nil {
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
	bytesUploadingCounter *atomic.Uint64,
) error {
	captureFileType := uploadMD.GetType()
	switch captureFileType {
	case datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR:
		// If it's a large binary file, we need to upload it in chunks.
		if uploadMD.GetType() == datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR && fileSize > MaxUnaryFileSize {
			return uploadMultipleLargeBinarySensorData(ctx, client, uploadMD, sensorData, path, logger, bytesUploadingCounter)
		}
		return uploadMultipleBinarySensorData(ctx, client, uploadMD, sensorData, path, logger, bytesUploadingCounter)
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
	bytesUploadingCounter *atomic.Uint64,
) error {
	// if the binary sensor data has a mime type, set the file extension
	// to match
	fileExtensionFromMimeType := getFileExtFromStringMimeType(md.GetMimeType())
	if fileExtensionFromMimeType != "" {
		md.FileExtension = fileExtensionFromMimeType
	}
	if _, err := client.DataCaptureUpload(ctx, &datasyncPB.DataCaptureUploadRequest{
		Metadata:       md,
		SensorContents: []*datasyncPB.SensorData{sd},
	}); err != nil {
		return errors.Wrap(err, "DataCaptureUpload failed")
	}

	// Count bytes uploaded.
	if bytesUploadingCounter != nil {
		bytesUploadingCounter.Add(uint64(len(sd.GetBinary())))
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
	bytesUploadingCounter *atomic.Uint64,
) error {
	// this is the common case
	if len(sensorData) == 1 {
		logger.Debugf("attempting to upload small binary file using DataCaptureUpload, sensor data, file: %s", path)
		return uploadBinarySensorData(ctx, client, uploadMD, sensorData[0], bytesUploadingCounter)
	}

	// we only go down this path if the capture method returned multiple binary
	// responses, which at time of writing, only includes camera.GetImages data.
	for i, sd := range sensorData {
		logger.Debugf("attempting to upload small binary file using DataCaptureUpload, sensor data index: %d, ext: %s, file: %s", i, path)
		// we clone as the uploadMD may be changed for each sensor data
		// and I'm not confident that it is safe to reuse grpc request structs
		// between calls if the data in the request struct changes
		clonedMD := proto.Clone(uploadMD).(*datasyncPB.UploadMetadata)
		if err := uploadBinarySensorData(ctx, client, clonedMD, sd, bytesUploadingCounter); err != nil {
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
	bytesUploadingCounter *atomic.Uint64,
) error {
	if len(sensorData) == 1 {
		logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, sensor data file: %s", path)
		return uploadLargeBinarySensorData(ctx, client, uploadMD, sensorData[0], path, logger, bytesUploadingCounter)
	}

	for i, sd := range sensorData {
		logger.Debugf("attempting to upload large binary file using StreamingDataCaptureUpload, sensor data index: %d, file: %s", i, path)
		// we clone as the uploadMD may be changed for each sensor data
		// and I'm not confident that it is safe to reuse grpc request structs
		// between calls if the data in the request struct changes
		clonedMD := proto.Clone(uploadMD).(*datasyncPB.UploadMetadata)
		if err := uploadLargeBinarySensorData(ctx, client, clonedMD, sd, path, logger, bytesUploadingCounter); err != nil {
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
	bytesUploadingCounter *atomic.Uint64,
) error {
	c, err := client.StreamingDataCaptureUpload(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating StreamingDataCaptureUpload client")
	}
	// if the binary sensor data has a mime type, set the file extension
	// to match
	smd := sd.GetMetadata()
	fileExtensionFromMimeType := getFileExtFromStringMimeType(md.GetMimeType())
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
	if err := sendStreamingDCRequests(ctx, c, bytes.NewReader(sd.GetBinary()), path, logger, bytesUploadingCounter); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed to sync")
	}

	if _, err = c.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload CloseAndRecv failed")
	}

	return nil
}

// uploadLargeBinaryFromReader streams the binary payload from r without loading it
// into memory. sensorMeta may be nil if the capture file had no SensorMetadata.
func uploadLargeBinaryFromReader(
	ctx context.Context,
	client datasyncPB.DataSyncServiceClient,
	md *datasyncPB.UploadMetadata,
	sensorMeta *datasyncPB.SensorMetadata,
	r io.Reader,
	path string,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) error {
	c, err := client.StreamingDataCaptureUpload(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating StreamingDataCaptureUpload client")
	}

	fileExtensionFromMimeType := getFileExtFromStringMimeType(md.GetMimeType())
	if fileExtensionFromMimeType != "" {
		md.FileExtension = fileExtensionFromMimeType
	}

	if err := c.Send(&datasyncPB.StreamingDataCaptureUploadRequest{
		UploadPacket: &datasyncPB.StreamingDataCaptureUploadRequest_Metadata{
			Metadata: &datasyncPB.DataCaptureUploadMetadata{
				UploadMetadata: md,
				SensorMetadata: sensorMeta,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "StreamingDataCaptureUpload failed sending metadata")
	}

	if err := sendStreamingDCRequests(ctx, c, r, path, logger, bytesUploadingCounter); err != nil {
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
	r io.Reader,
	path string,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) error {
	buf := make([]byte, UploadChunkSize)
	chunkCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := io.ReadFull(r, buf)
			if n > 0 {
				uploadReq := &datasyncPB.StreamingDataCaptureUploadRequest{
					UploadPacket: &datasyncPB.StreamingDataCaptureUploadRequest_Data{
						Data: buf[:n],
					},
				}
				logger.Debugf("datasync.StreamingDataCaptureUpload sending chunk %d for file: %s", chunkCount, path)
				if sendErr := stream.Send(uploadReq); sendErr != nil {
					return sendErr
				}
				if bytesUploadingCounter != nil {
					bytesUploadingCounter.Add(uint64(n))
				}
				chunkCount++
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

func getFileExtFromImageMimeType(mimeType string) string {
	switch mimeType {
	case utils.MimeTypeJPEG:
		return data.ExtJpeg
	case utils.MimeTypePNG:
		return data.ExtPng
	case utils.MimeTypeRawDepth:
		return ".dep"
	case utils.MimeTypeRawRGBA:
		return ".rgba"
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
	case datasyncPB.MimeType_MIME_TYPE_VIDEO_MP4:
		return data.ExtMP4
	case datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return data.ExtDefault
	}
}

func getFileExtFromStringMimeType(mimeType string) string {
	fileExt := getFileExtFromImageMimeType(mimeType)
	if fileExt != "" {
		return fileExt
	}
	switch mimeType {
	case utils.MimeTypePCD:
		return data.ExtPcd
	case utils.MimeTypeVideoMP4:
		return data.ExtMP4
	case utils.MimeTypeAudioWAV:
		return data.ExtWav
	case utils.MimeTypeAudioMPEG:
		return data.ExtMP3
	case utils.MimeTypeDefault:
		fallthrough
	default:
		return data.ExtDefault
	}
}
