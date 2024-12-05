package camera_test

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
	v1 "go.viam.com/api/app/data/v1"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

//nolint:lll
var viamLogoJpegB64 = []byte("/9j/4QD4RXhpZgAATU0AKgAAAAgABwESAAMAAAABAAEAAAEaAAUAAAABAAAAYgEbAAUAAAABAAAAagEoAAMAAAABAAIAAAExAAIAAAAhAAAAcgITAAMAAAABAAEAAIdpAAQAAAABAAAAlAAAAAAAAABIAAAAAQAAAEgAAAABQWRvYmUgUGhvdG9zaG9wIDIzLjQgKE1hY2ludG9zaCkAAAAHkAAABwAAAAQwMjIxkQEABwAAAAQBAgMAoAAABwAAAAQwMTAwoAEAAwAAAAEAAQAAoAIABAAAAAEAAAAgoAMABAAAAAEAAAAgpAYAAwAAAAEAAAAAAAAAAAAA/9sAhAAcHBwcHBwwHBwwRDAwMERcRERERFx0XFxcXFx0jHR0dHR0dIyMjIyMjIyMqKioqKioxMTExMTc3Nzc3Nzc3NzcASIkJDg0OGA0NGDmnICc5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ub/3QAEAAL/wAARCAAgACADASIAAhEBAxEB/8QBogAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoLEAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+foBAAMBAQEBAQEBAQEAAAAAAAABAgMEBQYHCAkKCxEAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDm6K0dNu1tZsSgGNuDx0961NX09WT7ZbgcD5gPT1oA5qiul0fT1VPtlwByPlB7D1rL1K7W5mxEAI04GBjPvQB//9Dm66TRr/I+xTf8A/wrm6ASpBXgjpQB0ms34UfYof8AgWP5VzdBJY5PJNFAH//Z")

const (
	serviceName     = "camera"
	captureInterval = time.Millisecond
)

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
	logger := logging.NewTestLogger(t)
	methodParams, err := convertStringMapToAnyPBMap(map[string]string{"camera_name": "camera-1", "mime_type": "image/jpeg"})
	test.That(t, err, test.ShouldBeNil)
	viamLogoJpeg, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(viamLogoJpegB64)))
	test.That(t, err, test.ShouldBeNil)

	img := rimage.NewLazyEncodedImage(viamLogoJpeg, utils.MimeTypeJPEG)
	// 32 x 32 image
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 32)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 32)

	pcd, err := pointcloud.NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)

	var pcdBuf bytes.Buffer
	test.That(t, pointcloud.ToPCD(pcd, &pcdBuf, pointcloud.PCDBinary), test.ShouldBeNil)

	cam := newCamera(img, img, pcd)

	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
		camera    camera.Camera
	}{
		{
			name:      "ReadImage returns a non nil binary response",
			collector: camera.NewReadImageCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
				},
				Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
			}},
			camera: cam,
		},
		{
			name:      "NextPointCloud returns a non nil binary response",
			collector: camera.NewNextPointCloudCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_APPLICATION_PCD,
				},
				Data: &datasyncpb.SensorData_Binary{Binary: pcdBuf.Bytes()},
			}},
			camera: cam,
		},
		{
			name:      "GetImages returns a non nil binary response",
			collector: camera.NewGetImagesCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{
						MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
						Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "left"}}},
					},
					Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
				},
				{
					Metadata: &datasyncpb.SensorMetadata{
						MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
						Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "right"}}},
					},
					Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
				},
			},
			camera: cam,
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

			col, err := tc.collector(tc.camera, params)
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

func newCamera(
	left, right image.Image,
	pcd pointcloud.PointCloud,
) camera.Camera {
	v := &inject.Camera{}
	v.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		viamLogoJpegBytes, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(viamLogoJpegB64)))
		if err != nil {
			return nil, camera.ImageMetadata{}, err
		}
		return viamLogoJpegBytes, camera.ImageMetadata{MimeType: mimeType}, nil
	}

	v.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcd, nil
	}

	v.ImagesFunc = func(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		return []camera.NamedImage{
				{Image: left, SourceName: "left"},
				{Image: right, SourceName: "right"},
			},
			resource.ResponseMetadata{CapturedAt: time.Now()},
			nil
	}

	return v
}
