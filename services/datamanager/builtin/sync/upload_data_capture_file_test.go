package sync

import (
	"context"
	"os"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/go-units"
	v1 "go.viam.com/api/app/datasync/v1"
	powersensorPB "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

func TestUploadDataCaptureFile(t *testing.T) {
	type upload struct {
		md *v1.UploadMetadata
		sd []*v1.SensorData
	}
	type testCase struct {
		testName         string
		api              resource.API
		name             string
		method           string
		tags             []string
		captureType      data.CaptureType
		captureResults   data.CaptureResult
		client           MockDataSyncServiceClient
		expectedUploads  []upload
		additionalParams map[string]interface{}
		unaryReqs        chan *v1.DataCaptureUploadRequest
		steamingReqs     []chan *v1.StreamingDataCaptureUploadRequest
	}

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	logger := logging.NewTestLogger(t)

	partID := "my-part-id"

	now := time.Now()
	ts := data.Timestamps{TimeRequested: now, TimeReceived: time.Now()}
	sensorReadingResult, err := data.NewTabularCaptureResultReadings(ts, map[string]interface{}{"a": 1})
	test.That(t, err, test.ShouldBeNil)

	ts1 := data.Timestamps{TimeRequested: now, TimeReceived: time.Now()}
	tabularResult, err := data.NewTabularCaptureResult(ts1, &powersensorPB.GetPowerResponse{Watts: 0.5})
	test.That(t, err, test.ShouldBeNil)

	ts2 := data.Timestamps{TimeRequested: now, TimeReceived: now.Add(time.Second)}
	smallBinaryJpegResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: []byte("I'm a small binary result"), MimeType: data.MimeTypeImageJpeg},
	})
	test.That(t, err, test.ShouldBeNil)

	smallBinaryPngResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: []byte("I'm a small binary result"), MimeType: data.MimeTypeImagePng},
	})
	test.That(t, err, test.ShouldBeNil)

	smallBinaryNoMimeTypeResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: []byte("I'm a small binary result")},
	})
	test.That(t, err, test.ShouldBeNil)

	largeBinaryPayload := slices.Repeat([]byte{1, 2}, units.MB)
	largeBinaryResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: largeBinaryPayload, MimeType: data.MimeTypeImagePng},
	})
	test.That(t, err, test.ShouldBeNil)

	largeBinaryNoMimeTypeResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: largeBinaryPayload},
	})
	test.That(t, err, test.ShouldBeNil)

	smallGetImagesResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: []byte("I'm a small binary jpeg result"), MimeType: data.MimeTypeImageJpeg},
		{Payload: []byte("I'm a small binary png result"), MimeType: data.MimeTypeImagePng},
	})

	largeGetImagesResult := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{Payload: largeBinaryPayload, MimeType: data.MimeTypeImageJpeg},
		{Payload: largeBinaryPayload, MimeType: data.MimeTypeImagePng},
	})
	conf := 0.888
	smallVisionCaptureAllFromCamera := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{
			Payload:  []byte("I'm a small binary jpeg result"),
			MimeType: data.MimeTypeImageJpeg,
			Annotations: data.Annotations{
				BoundingBoxes: []data.BoundingBox{
					{
						Label:          "a",
						Confidence:     &conf,
						XMinNormalized: 1,
						XMaxNormalized: 2,
						YMinNormalized: 3,
						YMaxNormalized: 4,
					},
					{
						Label:          "b",
						XMinNormalized: 5,
						XMaxNormalized: 6,
						YMinNormalized: 7,
						YMaxNormalized: 8,
					},
				},
				Classifications: []data.Classification{
					{Label: "a", Confidence: &conf},
					{Label: "b"},
				},
			},
		},
	})

	largeVisionCaptureAllFromCamera := data.NewBinaryCaptureResult(ts2, []data.Binary{
		{
			Payload:  largeBinaryPayload,
			MimeType: data.MimeTypeImagePng,
			Annotations: data.Annotations{
				BoundingBoxes: []data.BoundingBox{
					{
						Label:          "a",
						Confidence:     &conf,
						XMinNormalized: 1,
						XMaxNormalized: 2,
						YMinNormalized: 3,
						YMaxNormalized: 4,
					},
					{
						Label:          "b",
						XMinNormalized: 5,
						XMaxNormalized: 6,
						YMinNormalized: 7,
						YMaxNormalized: 8,
					},
				},
				Classifications: []data.Classification{
					{Label: "a", Confidence: &conf},
					{Label: "b"},
				},
			},
		},
	})

	reqs0 := make(chan *v1.DataCaptureUploadRequest, 1)
	reqs1 := make(chan *v1.DataCaptureUploadRequest, 1)
	reqs2 := make(chan *v1.DataCaptureUploadRequest, 1)
	largeReadImageReqs := []chan *v1.StreamingDataCaptureUploadRequest{
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
	}
	reqs4 := make(chan *v1.DataCaptureUploadRequest, 2)
	largeGetImagesReqsIdx := atomic.Int64{}
	largeGetImagesReqs := []chan *v1.StreamingDataCaptureUploadRequest{
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
	}
	reqs5 := make(chan *v1.DataCaptureUploadRequest, 2)
	largeVisionCaptureAllFromCameraIdx := atomic.Int64{}
	largeVisionCaptureAllFromCameraReqs := []chan *v1.StreamingDataCaptureUploadRequest{
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
	}
	reqs6 := make(chan *v1.DataCaptureUploadRequest, 1)
	largeReadImageNonMatchingMimeTypeReqs := []chan *v1.StreamingDataCaptureUploadRequest{
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
	}
	reqs7 := make(chan *v1.DataCaptureUploadRequest, 1)
	largeReadImageNoMimeTypeReqs := []chan *v1.StreamingDataCaptureUploadRequest{
		make(chan *v1.StreamingDataCaptureUploadRequest, 100),
	}

	//nolint:dupl
	tcs := []testCase{
		{
			testName:       "sensor readings",
			captureResults: sensorReadingResult,
			captureType:    data.CaptureTypeTabular,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs0 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:              sensor.API,
			name:             "sensor-1",
			method:           "Readings",
			tags:             []string{},
			additionalParams: map[string]interface{}{},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "sensor-1",
						ComponentType: sensor.API.String(),
						FileExtension: ".dat",
						MethodName:    "Readings",
						PartId:        partID,
						Type:          v1.DataType_DATA_TYPE_TABULAR_SENSOR,
					},
					sd: sensorReadingResult.ToProto(),
				},
			},
			unaryReqs: reqs0,
		},
		{
			testName:       "non readings tabular data",
			captureResults: tabularResult,
			captureType:    data.CaptureTypeTabular,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs1 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:              powersensor.API,
			name:             "powersensor-1",
			method:           "Power",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"some": "additional", "param": "things"},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "powersensor-1",
						ComponentType: powersensor.API.String(),
						FileExtension: ".dat",
						MethodName:    "Power",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_TABULAR_SENSOR,
					},
					sd: tabularResult.ToProto(),
				},
			},
			unaryReqs: reqs1,
		},
		{
			testName:       "small binary data",
			captureResults: smallBinaryJpegResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs2 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:              camera.API,
			name:             "camera-1",
			method:           "ReadImage",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypeJPEG,
					},
					sd: smallBinaryJpegResult.ToProto(),
				},
			},
			unaryReqs: reqs2,
		},
		{
			testName:       "small binary when additional params mime type doesn't match collector response",
			captureResults: smallBinaryPngResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs6 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:    camera.API,
			name:   "camera-1",
			method: "ReadImage",
			tags:   []string{"tag1", "tag2"},
			// note additional params specify jpeg but collector returns png,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						// note additional params specify jpeg but collector returns png,
						// file extension should match what the collector output
						FileExtension: ".png",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: smallBinaryPngResult.ToProto(),
				},
			},
			unaryReqs: reqs6,
		},
		{
			testName:       "big binary when additional params mime type doesn't match collector response",
			captureResults: largeBinaryResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				StreamingDataCaptureUploadFunc: func(
					ctx context.Context,
					_ ...grpc.CallOption,
				) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
					mockStreamingClient := &ClientStreamingMock[
						*v1.StreamingDataCaptureUploadRequest,
						*v1.StreamingDataCaptureUploadResponse,
					]{
						T: t,
						SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
							select {
							case <-testCtx.Done():
								t.Error("timeout")
								t.FailNow()
							case largeReadImageNonMatchingMimeTypeReqs[0] <- in:
							}
							return nil
						},
						CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
							close(largeReadImageNonMatchingMimeTypeReqs[0])
							return &v1.StreamingDataCaptureUploadResponse{}, nil
						},
					}
					return mockStreamingClient, nil
				},
			},
			api:              camera.API,
			name:             "camera-1",
			method:           "ReadImage",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".png",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: largeBinaryResult.ToProto(),
				},
			},
			steamingReqs: largeReadImageNonMatchingMimeTypeReqs,
		},
		{
			testName:       "small binary when collector response doesn't specify mime_type defaults to FileExtension",
			captureResults: smallBinaryNoMimeTypeResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs7 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:              camera.API,
			name:             "camera-1",
			method:           "ReadImage",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
					},
					sd: smallBinaryNoMimeTypeResult.ToProto(),
				},
			},
			unaryReqs: reqs7,
		},
		{
			testName:       "big binary when collector response doesn't specify mime_type defaults to FileExtension",
			captureResults: largeBinaryNoMimeTypeResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				StreamingDataCaptureUploadFunc: func(
					ctx context.Context,
					_ ...grpc.CallOption,
				) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
					mockStreamingClient := &ClientStreamingMock[
						*v1.StreamingDataCaptureUploadRequest,
						*v1.StreamingDataCaptureUploadResponse,
					]{
						T: t,
						SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
							select {
							case <-testCtx.Done():
								t.Error("timeout")
								t.FailNow()
							case largeReadImageNoMimeTypeReqs[0] <- in:
							}
							return nil
						},
						CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
							close(largeReadImageNoMimeTypeReqs[0])
							return &v1.StreamingDataCaptureUploadResponse{}, nil
						},
					}
					return mockStreamingClient, nil
				},
			},
			api:              camera.API,
			name:             "camera-1",
			method:           "ReadImage",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
					},
					sd: largeBinaryNoMimeTypeResult.ToProto(),
				},
			},
			steamingReqs: largeReadImageNoMimeTypeReqs,
		},
		{
			testName:       "large binary data",
			captureResults: largeBinaryResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				StreamingDataCaptureUploadFunc: func(
					ctx context.Context,
					_ ...grpc.CallOption,
				) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
					mockStreamingClient := &ClientStreamingMock[
						*v1.StreamingDataCaptureUploadRequest,
						*v1.StreamingDataCaptureUploadResponse,
					]{
						T: t,
						SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
							select {
							case <-testCtx.Done():
								t.Error("timeout")
								t.FailNow()
							case largeReadImageReqs[0] <- in:
							}
							return nil
						},
						CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
							close(largeReadImageReqs[0])
							return &v1.StreamingDataCaptureUploadResponse{}, nil
						},
					}
					return mockStreamingClient, nil
				},
			},
			api:              camera.API,
			name:             "camera-1",
			method:           "ReadImage",
			tags:             []string{"tag1", "tag2"},
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypePNG},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".png",
						MethodName:    "ReadImage",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: largeBinaryResult.ToProto(),
				},
			},
			steamingReqs: largeReadImageReqs,
		},
		{
			testName:       "small camera.GetImages",
			captureResults: smallGetImagesResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs4 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:    camera.API,
			name:   "camera-1",
			method: "GetImages",
			tags:   []string{"tag1", "tag2"},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "GetImages",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypeJPEG,
					},
					sd: []*v1.SensorData{smallGetImagesResult.ToProto()[0]},
				},
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".png",
						MethodName:    "GetImages",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: []*v1.SensorData{smallGetImagesResult.ToProto()[1]},
				},
			},
			unaryReqs: reqs4,
		},
		{
			testName:       "large camera.GetImages",
			captureResults: largeGetImagesResult,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				StreamingDataCaptureUploadFunc: func(
					ctx context.Context,
					_ ...grpc.CallOption,
				) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
					mockStreamingClient := &ClientStreamingMock[
						*v1.StreamingDataCaptureUploadRequest,
						*v1.StreamingDataCaptureUploadResponse,
					]{
						T: t,
						SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
							idx := largeGetImagesReqsIdx.Load()
							t.Logf("writing to index: %d", idx)
							ch := largeGetImagesReqs[idx]
							select {
							case <-testCtx.Done():
								t.Error("timeout")
								t.FailNow()
							case ch <- in:
							}
							return nil
						},
						CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
							close(largeGetImagesReqs[largeGetImagesReqsIdx.Add(1)-1])
							return &v1.StreamingDataCaptureUploadResponse{}, nil
						},
					}
					return mockStreamingClient, nil
				},
			},
			api:    camera.API,
			name:   "camera-1",
			method: "GetImages",
			tags:   []string{"tag1", "tag2"},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "GetImages",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypeJPEG,
					},
					sd: []*v1.SensorData{largeGetImagesResult.ToProto()[0]},
				},
				{
					md: &v1.UploadMetadata{
						ComponentName: "camera-1",
						ComponentType: camera.API.String(),
						FileExtension: ".png",
						MethodName:    "GetImages",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: []*v1.SensorData{largeGetImagesResult.ToProto()[1]},
				},
			},
			steamingReqs: largeGetImagesReqs,
		},
		{
			testName:       "small vision.CaptureAllFromCamera",
			captureResults: smallVisionCaptureAllFromCamera,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				DataCaptureUploadFunc: func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					t.Log("called")
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case reqs5 <- in:
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			},
			api:    vision.API,
			name:   "vision-1",
			method: "CaptureAllFromCamera",
			tags:   []string{"tag1", "tag2"},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "vision-1",
						ComponentType: vision.API.String(),
						FileExtension: ".jpeg",
						MethodName:    "CaptureAllFromCamera",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypeJPEG,
					},
					sd: smallVisionCaptureAllFromCamera.ToProto(),
				},
			},
			unaryReqs: reqs5,
		},
		{
			testName:       "large vision.CaptureAllFromCamera",
			captureResults: largeVisionCaptureAllFromCamera,
			captureType:    data.CaptureTypeBinary,
			client: MockDataSyncServiceClient{
				T: t,
				StreamingDataCaptureUploadFunc: func(
					ctx context.Context,
					_ ...grpc.CallOption,
				) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
					mockStreamingClient := &ClientStreamingMock[
						*v1.StreamingDataCaptureUploadRequest,
						*v1.StreamingDataCaptureUploadResponse,
					]{
						T: t,
						SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
							idx := largeVisionCaptureAllFromCameraIdx.Load()
							t.Logf("writing to index: %d", idx)
							ch := largeVisionCaptureAllFromCameraReqs[idx]
							select {
							case <-testCtx.Done():
								t.Error("timeout")
								t.FailNow()
							case ch <- in:
							}
							return nil
						},
						CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
							close(largeVisionCaptureAllFromCameraReqs[largeVisionCaptureAllFromCameraIdx.Add(1)-1])
							return &v1.StreamingDataCaptureUploadResponse{}, nil
						},
					}
					return mockStreamingClient, nil
				},
			},
			api:    vision.API,
			name:   "vision-1",
			method: "CaptureAllFromCamera",
			tags:   []string{"tag1", "tag2"},
			expectedUploads: []upload{
				{
					md: &v1.UploadMetadata{
						ComponentName: "vision-1",
						ComponentType: vision.API.String(),
						FileExtension: ".png",
						MethodName:    "CaptureAllFromCamera",
						PartId:        partID,
						Tags:          []string{"tag1", "tag2"},
						Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
						MimeType:      utils.MimeTypePNG,
					},
					sd: largeVisionCaptureAllFromCamera.ToProto(),
				},
			},
			steamingReqs: largeVisionCaptureAllFromCameraReqs,
		},
	}

	tempDir := t.TempDir()
	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			methodParams, err := rprotoutils.ConvertMapToProtoAny(tc.additionalParams)
			test.That(t, err, test.ShouldBeNil)
			md, ct := data.BuildCaptureMetadata(tc.api, tc.name, tc.method, tc.additionalParams, methodParams, tc.tags)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ct, test.ShouldEqual, tc.captureType)
			test.That(t, len(tc.expectedUploads), test.ShouldBeGreaterThan, 0)

			protoData := tc.captureResults.ToProto()

			// Create and upload capture files
			// For multiple binaries (e.g., GetImages), create one file per binary
			// Otherwise, create one file with all data
			numFiles := 1
			if ct == data.CaptureTypeBinary && len(tc.captureResults.Binaries) > 1 {
				numFiles = len(tc.captureResults.Binaries)
			}

			for i := 0; i < numFiles; i++ {
				// Set MimeType and FileExtension from binary data if available
				if ct == data.CaptureTypeBinary && len(tc.captureResults.Binaries) > 0 {
					mimeType := tc.captureResults.Binaries[i].MimeType
					md.MimeType = mimeType.ToString()
					// Derive FileExtension from MimeType (if MimeType is set)
					if mimeType != data.MimeTypeUnspecified {
						md.FileExtension = getFileExtFromMimeType(mimeType.ToProto())
					}
				}

				// Create and write file
				w, err := data.NewCaptureFile(tempDir, md)
				test.That(t, err, test.ShouldBeNil)
				if numFiles > 1 {
					test.That(t, w.WriteNext(protoData[i]), test.ShouldBeNil)
				} else {
					for _, sd := range protoData {
						test.That(t, w.WriteNext(sd), test.ShouldBeNil)
					}
				}
				w.Flush()
				w.Close()

				// Upload file
				f, err := os.Open(strings.Replace(w.GetPath(), data.InProgressCaptureFileExt, data.CompletedCaptureFileExt, 1))
				test.That(t, err, test.ShouldBeNil)
				stat, err := f.Stat()
				test.That(t, err, test.ShouldBeNil)

				test.That(t, data.IsDataCaptureFile(f), test.ShouldBeTrue)
				cf, err := data.ReadCaptureFile(f)
				test.That(t, err, test.ShouldBeNil)
				cc := cloudConn{partID: partID, client: tc.client}
				bytesUploaded, err := uploadDataCaptureFile(testCtx, cf, cc, logger, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, bytesUploaded, test.ShouldEqual, stat.Size())
			}

			if tc.unaryReqs != nil {
				for i := 0; i < len(tc.expectedUploads); i++ {
					t.Logf("unaryReqs: i: %d", i)
					tc.expectedUploads[i].md.MethodParameters = methodParams
					select {
					case <-testCtx.Done():
						t.Error("timeout")
						t.FailNow()
					case req := <-tc.unaryReqs:
						t.Logf("got req\n")
						test.That(t, len(tc.expectedUploads[i].sd), test.ShouldEqual, 1)
						test.That(t, req.Metadata.PartId, test.ShouldResemble, tc.expectedUploads[i].md.PartId)
						test.That(t, req.Metadata.ComponentType, test.ShouldResemble, tc.expectedUploads[i].md.ComponentType)
						test.That(t, req.Metadata.ComponentName, test.ShouldResemble, tc.expectedUploads[i].md.ComponentName)
						test.That(t, req.Metadata.MethodName, test.ShouldResemble, tc.expectedUploads[i].md.MethodName)
						test.That(t, req.Metadata.Type, test.ShouldResemble, tc.expectedUploads[i].md.Type)
						test.That(t, req.Metadata.Tags, test.ShouldResemble, tc.expectedUploads[i].md.Tags)
						test.That(t, req.Metadata.FileExtension, test.ShouldResemble, tc.expectedUploads[i].md.FileExtension)
						test.That(t, req.Metadata.MimeType, test.ShouldResemble, tc.expectedUploads[i].md.MimeType)
						compareSensorData(t, tc.captureType.ToProto(), req.SensorContents, tc.expectedUploads[i].sd)
					}
				}
			} else {
				test.That(t, len(tc.steamingReqs), test.ShouldEqual, len(tc.expectedUploads))
				for i := 0; i < len(tc.expectedUploads); i++ {
					test.That(t, len(tc.expectedUploads[i].sd), test.ShouldEqual, 1)
					md := tc.expectedUploads[i].md
					sd := tc.expectedUploads[i].sd[0]
					md.MethodParameters = methodParams
					var gotHeader bool
					var data []byte
					for req := range tc.steamingReqs[i] {
						if !gotHeader {
							actualMD := req.GetMetadata().UploadMetadata
							test.That(t, actualMD.PartId, test.ShouldResemble, md.PartId)
							test.That(t, actualMD.ComponentType, test.ShouldResemble, md.ComponentType)
							test.That(t, actualMD.ComponentName, test.ShouldResemble, md.ComponentName)
							test.That(t, actualMD.MethodName, test.ShouldResemble, md.MethodName)
							test.That(t, actualMD.Type, test.ShouldResemble, md.Type)
							test.That(t, actualMD.Tags, test.ShouldResemble, md.Tags)
							test.That(t, actualMD.FileExtension, test.ShouldResemble, md.FileExtension)
							test.That(t, actualMD.MimeType, test.ShouldResemble, md.MimeType)
							test.That(t, req.GetMetadata().SensorMetadata, test.ShouldResemble, sd.GetMetadata())
							gotHeader = true
							continue
						}
						data = append(data, req.GetData()...)
					}
					test.That(t, gotHeader, test.ShouldBeTrue)
					test.That(t, data, test.ShouldResemble, sd.GetBinary())
				}
			}
		})
	}
}

func compareSensorData(t *testing.T, dataType v1.DataType, act, exp []*v1.SensorData) {
	t.Helper()
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	// Sort both by time requested.
	sort.SliceStable(act, func(i, j int) bool {
		diffRequested := act[j].GetMetadata().GetTimeRequested().AsTime().Sub(act[i].GetMetadata().GetTimeRequested().AsTime())
		switch {
		case diffRequested > 0:
			return true
		case diffRequested == 0:
			return act[j].GetMetadata().GetTimeReceived().AsTime().Sub(act[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		default:
			return false
		}
	})
	sort.SliceStable(exp, func(i, j int) bool {
		diffRequested := exp[j].GetMetadata().GetTimeRequested().AsTime().Sub(exp[i].GetMetadata().GetTimeRequested().AsTime())
		switch {
		case diffRequested > 0:
			return true
		case diffRequested == 0:
			return exp[j].GetMetadata().GetTimeReceived().AsTime().Sub(exp[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		default:
			return false
		}
	})

	test.That(t, len(act), test.ShouldEqual, len(exp))

	for i := range act {
		test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
		if dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
			test.That(t, act[i].GetStruct(), test.ShouldResemble, exp[i].GetStruct())
		} else {
			test.That(t, act[i].GetBinary(), test.ShouldResemble, exp[i].GetBinary())
		}
	}
}
