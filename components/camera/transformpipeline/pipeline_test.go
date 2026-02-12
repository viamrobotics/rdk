package transformpipeline

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

// staticSourceCamera wraps a fake.StaticSource to implement camera.Camera.
type staticSourceCamera struct {
	resource.Named
	resource.AlwaysRebuild
	fake.StaticSource
	stream camera.ImageType
}

func (s *staticSourceCamera) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{ImageType: s.stream}, nil
}

func (s *staticSourceCamera) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (s *staticSourceCamera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, resource.ErrDoUnimplemented
}

func TestTransformPipelineColor(t *testing.T) {
	transformConf := &transformConfig{
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: utils.AttributeMap{}},
			{Type: "resize", Attributes: utils.AttributeMap{"height_px": 20, "width_px": 10}},
		},
	}
	r := &inject.Robot{}
	logger := logging.NewTestLogger(t)

	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)
	src := &staticSourceCamera{StaticSource: fake.StaticSource{ColorImg: img}, stream: camera.ColorStream}
	inImg, err := camera.DecodeImageFromCamera(context.Background(), src, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inImg.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, inImg.Bounds().Dy(), test.ShouldEqual, 72)

	color, err := newTransformPipeline(context.Background(), src, nil, transformConf, r, logger)
	test.That(t, err, test.ShouldBeNil)

	outImg, err := camera.DecodeImageFromCamera(context.Background(), color, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 10)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 20)
	_, err = color.NextPointCloud(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)

	test.That(t, color.Close(context.Background()), test.ShouldBeNil)
}

func TestTransformPipelineDepth(t *testing.T) {
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  128,
		Height: 72,
		Fx:     900.538000,
		Fy:     900.818000,
		Ppx:    64.8934000,
		Ppy:    36.7736000,
	}

	transformConf := &transformConfig{
		CameraParameters: intrinsics,
		Source:           "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: utils.AttributeMap{}},
			{Type: "resize", Attributes: utils.AttributeMap{"height_px": 30, "width_px": 40}},
		},
	}
	r := &inject.Robot{}
	logger := logging.NewTestLogger(t)

	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)
	src := &staticSourceCamera{StaticSource: fake.StaticSource{DepthImg: dm}, stream: camera.DepthStream}
	inImg, err := camera.DecodeImageFromCamera(context.Background(), src, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inImg.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, inImg.Bounds().Dy(), test.ShouldEqual, 72)

	depth, err := newTransformPipeline(context.Background(), src, nil, transformConf, r, logger)
	test.That(t, err, test.ShouldBeNil)

	outImg, err := camera.DecodeImageFromCamera(context.Background(), depth, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 40)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 30)
	prop, err := depth.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams, test.ShouldResemble, intrinsics)
	outPc, err := depth.NextPointCloud(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not defined for last videosource")
	test.That(t, outPc, test.ShouldBeNil)

	test.That(t, depth.Close(context.Background()), test.ShouldBeNil)
}

func TestNullPipeline(t *testing.T) {
	transform1 := &transformConfig{
		Source:   "source",
		Pipeline: []Transformation{},
	}
	r := &inject.Robot{}
	logger := logging.NewTestLogger(t)

	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &staticSourceCamera{StaticSource: fake.StaticSource{ColorImg: img}, stream: camera.UnspecifiedStream}
	_, err = newTransformPipeline(context.Background(), source, nil, transform1, r, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "pipeline has no transforms")
}

func TestPipeIntoPipe(t *testing.T) {
	r := &inject.Robot{}
	logger := logging.NewTestLogger(t)

	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &staticSourceCamera{StaticSource: fake.StaticSource{ColorImg: img}, stream: camera.UnspecifiedStream}

	intrinsics1 := &transform.PinholeCameraIntrinsics{Width: 128, Height: 72}
	transform1 := &transformConfig{
		CameraParameters: intrinsics1,
		Source:           "source",
		Pipeline:         []Transformation{{Type: "rotate", Attributes: utils.AttributeMap{}}},
	}
	intrinsics2 := &transform.PinholeCameraIntrinsics{Width: 10, Height: 20}
	transform2 := &transformConfig{
		CameraParameters: intrinsics2,
		Source:           "transform2",
		Pipeline: []Transformation{
			{Type: "resize", Attributes: utils.AttributeMap{"height_px": 20, "width_px": 10}},
		},
	}

	pipe1, err := newTransformPipeline(context.Background(), source, nil, transform1, r, logger)
	test.That(t, err, test.ShouldBeNil)
	outImg, err := camera.DecodeImageFromCamera(context.Background(), pipe1, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 72)
	prop, err := pipe1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams.Width, test.ShouldEqual, 128)
	test.That(t, prop.IntrinsicParams.Height, test.ShouldEqual, 72)
	// transform pipeline into pipeline
	pipe2, err := newTransformPipeline(context.Background(), pipe1, nil, transform2, r, logger)
	test.That(t, err, test.ShouldBeNil)
	outImg, err = camera.DecodeImageFromCamera(context.Background(), pipe2, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 10)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 20)
	test.That(t, err, test.ShouldBeNil)
	prop, err = pipe2.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.IntrinsicParams.Width, test.ShouldEqual, 10)
	test.That(t, prop.IntrinsicParams.Height, test.ShouldEqual, 20)
	// Close everything
	test.That(t, pipe2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, pipe1.Close(context.Background()), test.ShouldBeNil)
}

func TestTransformPipelineValidatePass(t *testing.T) {
	transformConf := &transformConfig{
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: utils.AttributeMap{}},
			{Type: "resize", Attributes: utils.AttributeMap{"height_px": 20, "width_px": 10}},
		},
	}
	deps, _, err := transformConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{"source"})
}

func TestTransformPipelineValidateFail(t *testing.T) {
	transformConf := &transformConfig{
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: utils.AttributeMap{}},
			{Type: "resize", Attributes: utils.AttributeMap{"height_px": 20, "width_px": 10}},
		},
	}
	path := "path"
	deps, _, err := transformConf.Validate(path)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "source")
	test.That(t, deps, test.ShouldBeNil)
}

// Ensure staticSourceCamera implements camera.Camera at compile time.
var _ camera.Camera = (*staticSourceCamera)(nil)

// Ensure staticSourceCamera also implements PointCloudSource when the underlying StaticSource does.
var _ camera.PointCloudSource = (*staticSourceCamera)(nil)

// Verify NextPointCloud is delegated to the embedded StaticSource.
func TestStaticSourceCameraNextPointCloud(t *testing.T) {
	src := &staticSourceCamera{
		StaticSource: fake.StaticSource{ColorImg: image.NewRGBA(image.Rect(0, 0, 10, 10))},
	}
	_, err := src.NextPointCloud(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
}
