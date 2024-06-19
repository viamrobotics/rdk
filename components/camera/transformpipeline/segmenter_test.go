package transformpipeline

import (
	"context"
	"image"
	"image/color"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	segment "go.viam.com/rdk/vision/segmentation"
)

func TestTransformSegmenterProps(t *testing.T) {
	cam := &inject.Camera{}
	vizServ := &inject.VisionService{}
	logger := logging.NewTestLogger(t)

	cam.StreamFunc = func(ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.MediaStream[image.Image], error) {
		return &streamTest{}, nil
	}
	cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	fakeCameraName := resource.Name{Name: "fakeCamera", API: camera.API}
	fakeVizServName := resource.Name{Name: "fakeVizService", API: vizServ.Name().API}
	deps := resource.Dependencies{fakeCameraName: cam, fakeVizServName: vizServ}
	transformConf := &transformConfig{
		Source: "fakeCamera",
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{
					"segmenter_name": "fakeVizService",
				},
			},
		},
	}

	am := transformConf.Pipeline[0].Attributes
	conf, err := resource.TransformAttributeMap[*segmenterConfig](am)
	test.That(t, err, test.ShouldBeNil)
	_, err = conf.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	_, err = newTransformPipeline(context.Background(), cam, transformConf, deps, logger)
	test.That(t, err, test.ShouldBeNil)

	transformConf = &transformConfig{
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{},
			},
		},
	}

	am = transformConf.Pipeline[0].Attributes
	conf, err = resource.TransformAttributeMap[*segmenterConfig](am)
	test.That(t, err, test.ShouldBeNil)
	_, err = conf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestTransformSegmenterFunctionality(t *testing.T) {
	// TODO(RSDK-1200): remove skip when complete
	t.Skip("remove skip once RSDK-1200 improvement is complete")

	cam := &inject.Camera{}
	vizServ := &inject.VisionService{}
	logger := logging.NewTestLogger(t)

	cam.StreamFunc = func(ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.MediaStream[image.Image], error) {
		return &streamTest{}, nil
	}
	cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	vizServ.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string,
		extra map[string]interface{},
	) ([]*vision.Object, error) {
		segments := make([]pc.PointCloud, 3)
		segments[0] = pc.New()
		err := segments[0].Set(pc.NewVector(0, 0, 1), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}
		segments[1] = pc.New()
		err = segments[1].Set(pc.NewVector(0, 1, 0), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}
		segments[2] = pc.New()
		err = segments[2].Set(pc.NewVector(1, 0, 0), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}

		objects, err := segment.NewSegmentsFromSlice(segments, "fake")
		if err != nil {
			return nil, err
		}
		return objects.Objects, nil
	}

	fakeCameraName := resource.Name{Name: "fakeCamera", API: camera.API}
	fakeVizServName := resource.Name{Name: "fakeVizService", API: vizServ.Name().API}
	deps := resource.Dependencies{fakeCameraName: cam, fakeVizServName: vizServ}
	transformConf := &transformConfig{
		Source: "fakeCamera",
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{
					"segmenter_name": "fakeVizService",
				},
			},
		},
	}

	pipeline, err := newTransformPipeline(context.Background(), cam, transformConf, deps, logger)
	test.That(t, err, test.ShouldBeNil)

	pc, err := pipeline.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	_, isValid := pc.At(0, 0, 1)
	test.That(t, isValid, test.ShouldBeTrue)
	_, isValid = pc.At(1, 0, 0)
	test.That(t, isValid, test.ShouldBeTrue)
	_, isValid = pc.At(0, 1, 0)
	test.That(t, isValid, test.ShouldBeTrue)
}
