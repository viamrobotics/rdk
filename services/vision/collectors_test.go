package vision_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datapb "go.viam.com/api/app/data/v1"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	visionservice "go.viam.com/rdk/services/vision"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/viscapture"
)

//nolint:lll
var viamLogoJpegB64 = []byte("/9j/4QD4RXhpZgAATU0AKgAAAAgABwESAAMAAAABAAEAAAEaAAUAAAABAAAAYgEbAAUAAAABAAAAagEoAAMAAAABAAIAAAExAAIAAAAhAAAAcgITAAMAAAABAAEAAIdpAAQAAAABAAAAlAAAAAAAAABIAAAAAQAAAEgAAAABQWRvYmUgUGhvdG9zaG9wIDIzLjQgKE1hY2ludG9zaCkAAAAHkAAABwAAAAQwMjIxkQEABwAAAAQBAgMAoAAABwAAAAQwMTAwoAEAAwAAAAEAAQAAoAIABAAAAAEAAAAgoAMABAAAAAEAAAAgpAYAAwAAAAEAAAAAAAAAAAAA/9sAhAAcHBwcHBwwHBwwRDAwMERcRERERFx0XFxcXFx0jHR0dHR0dIyMjIyMjIyMqKioqKioxMTExMTc3Nzc3Nzc3NzcASIkJDg0OGA0NGDmnICc5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ub/3QAEAAL/wAARCAAgACADASIAAhEBAxEB/8QBogAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoLEAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+foBAAMBAQEBAQEBAQEAAAAAAAABAgMEBQYHCAkKCxEAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDm6K0dNu1tZsSgGNuDx0961NX09WT7ZbgcD5gPT1oA5qiul0fT1VPtlwByPlB7D1rL1K7W5mxEAI04GBjPvQB//9Dm66TRr/I+xTf8A/wrm6ASpBXgjpQB0ms34UfYof8AgWP5VzdBJY5PJNFAH//Z")

type fakeDetection struct {
	boundingBox *image.Rectangle
	score       float64
	label       string
}

type fakeClassification struct {
	score float64
	label string
}

const (
	serviceName     = "vision"
	captureInterval = time.Millisecond
)

var fakeDetections = []objectdetection.Detection{
	&fakeDetection{
		boundingBox: &image.Rectangle{
			Min: image.Point{X: 10, Y: 20},
			Max: image.Point{X: 110, Y: 120},
		},
		score: 0.95,
		label: "cat",
	},
}

var fakeDetections2 = []objectdetection.Detection{
	&fakeDetection{
		boundingBox: &image.Rectangle{
			Min: image.Point{X: 10, Y: 20},
			Max: image.Point{X: 110, Y: 120},
		},
		score: 0.3,
		label: "cat",
	},
}

var fakeClassifications = []classification.Classification{
	&fakeClassification{
		score: 0.85,
		label: "cat",
	},
}

var fakeClassifications2 = []classification.Classification{
	&fakeClassification{
		score: 0.49,
		label: "cat",
	},
}

func (fc *fakeClassification) Score() float64 {
	return fc.score
}

func (fc *fakeClassification) Label() string {
	return fc.label
}

func (fd *fakeDetection) BoundingBox() *image.Rectangle {
	return fd.boundingBox
}

func (fd *fakeDetection) Score() float64 {
	return fd.score
}

func (fd *fakeDetection) Label() string {
	return fd.label
}

func convertStringMapToAnyPBMap(params map[string]string) (map[string]*anypb.Any, error) {
	methodParams := map[string]*anypb.Any{}
	for key, paramVal := range params {
		anyVal, err := convertStringToAnyPB(paramVal)
		if err != nil {
			return nil, err
		}
		methodParams[key] = anyVal
	}
	return methodParams, nil
}

func convertStringToAnyPB(str string) (*anypb.Any, error) {
	var wrappedVal protoreflect.ProtoMessage
	if boolVal, err := strconv.ParseBool(str); err == nil {
		wrappedVal = wrapperspb.Bool(boolVal)
	} else if int64Val, err := strconv.ParseInt(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.Int64(int64Val)
	} else if uint64Val, err := strconv.ParseUint(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.UInt64(uint64Val)
	} else if float64Val, err := strconv.ParseFloat(str, 64); err == nil {
		wrappedVal = wrapperspb.Double(float64Val)
	} else {
		wrappedVal = wrapperspb.String(str)
	}
	anyVal, err := anypb.New(wrappedVal)
	if err != nil {
		return nil, err
	}
	return anyVal, nil
}

func TestCollectors(t *testing.T) {
	methodParams, err := convertStringMapToAnyPBMap(map[string]string{"camera_name": "camera-1"})
	test.That(t, err, test.ShouldBeNil)
	viamLogoJpeg, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(viamLogoJpegB64)))
	test.That(t, err, test.ShouldBeNil)
	img := rimage.NewLazyEncodedImage(viamLogoJpeg, utils.MimeTypeJPEG)
	// 32 x 32 image
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 32)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 32)
	bboxConf := 0.95
	classConf := 0.85
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
		vision    visionservice.Service
	}{
		{
			name:      "CaptureAllFromCameraCollector returns non-empty CaptureAllFromCameraResp",
			collector: visionservice.NewCaptureAllFromCameraCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
					Annotations: &datapb.Annotations{
						Bboxes: []*datapb.BoundingBox{
							{
								Label:          "cat",
								XMinNormalized: 0.3125,
								YMinNormalized: 0.625,
								XMaxNormalized: 3.4375,
								YMaxNormalized: 3.75,
								Confidence:     &bboxConf,
							},
						},
						Classifications: []*datapb.Classification{{
							Label:      "cat",
							Confidence: &classConf,
						}},
					},
				},
				Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
			}},
			vision: newVisionService(img),
		},
		{
			name:      "CaptureAllFromCameraCollector w/ Classifications & Detections < 0.5 returns empty CaptureAllFromCameraResp",
			collector: visionservice.NewCaptureAllFromCameraCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
				},
				Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
			}},
			vision: newVisionService2(img),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeBinary,
				ComponentName: serviceName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				MethodParams:  methodParams,
			}

			col, err := tc.collector(tc.vision, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
			buf.Close()
		})
	}
}

func newVisionService(img image.Image) visionservice.Service {
	v := &inject.VisionService{}
	v.CaptureAllFromCameraFunc = func(ctx context.Context, cameraName string, opts viscapture.CaptureOptions,
		extra map[string]interface{},
	) (viscapture.VisCapture, error) {
		return viscapture.VisCapture{
			Image:           img,
			Detections:      fakeDetections,
			Classifications: fakeClassifications,
		}, nil
	}

	return v
}

func newVisionService2(img image.Image) visionservice.Service {
	v := &inject.VisionService{}
	v.CaptureAllFromCameraFunc = func(ctx context.Context, cameraName string, opts viscapture.CaptureOptions,
		extra map[string]interface{},
	) (viscapture.VisCapture, error) {
		return viscapture.VisCapture{
			Image:           img,
			Detections:      fakeDetections2,
			Classifications: fakeClassifications2,
		}, nil
	}

	return v
}
