package transformpipeline

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
)

func TestTransformPipelineColor(t *testing.T) {
	transformConf := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "color",
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}
	r := &inject.Robot{}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	inImg, _, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, inImg.Bounds().Dy(), test.ShouldEqual, 720)

	color, err := newTransformPipeline(context.Background(), cam, transformConf, r)
	test.That(t, err, test.ShouldBeNil)

	outImg, _, err := color.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 100)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 200)
	_, err = color.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	_, err = color.GetProperties(context.Background())
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
}

func TestTransformPipelineDepth(t *testing.T) {
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     900.538000,
		Fy:     900.818000,
		Ppx:    648.934000,
		Ppy:    367.736000,
	}

	transformConf := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream:           "depth",
			CameraParameters: intrinsics,
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 300, "width": 400}},
		},
	}
	r := &inject.Robot{}

	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: dm}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	inImg, _, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, inImg.Bounds().Dy(), test.ShouldEqual, 720)

	depth, err := newTransformPipeline(context.Background(), cam, transformConf, r)
	test.That(t, err, test.ShouldBeNil)

	outImg, _, err := depth.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 400)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 300)
	prop, err := depth.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop, test.ShouldResemble, intrinsics)
	outPc, err := depth.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outPc, test.ShouldNotBeNil)
}

func TestTransformPipelineDepth2(t *testing.T) {
	transform1 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "depth_preprocess", Attributes: config.AttributeMap{}},
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
			{Type: "depth_to_pretty", Attributes: config.AttributeMap{}},
		},
	}
	transform2 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "depth_preprocess", Attributes: config.AttributeMap{}},
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 300, "width": 400}},
			{Type: "depth_edges", Attributes: config.AttributeMap{"high_threshold": 0.85, "low_threshold": 0.3, "blur_radius": 3.0}},
		},
	}
	r := &inject.Robot{}

	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: dm}
	// first depth transform
	depth1, err := newTransformPipeline(context.Background(), source, transform1, r)
	test.That(t, err, test.ShouldBeNil)
	outImg, _, err := depth1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 100)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 200)
	// second depth image
	depth2, err := newTransformPipeline(context.Background(), source, transform2, r)
	test.That(t, err, test.ShouldBeNil)
	outImg, _, err = depth2.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 400)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 300)
}

func TestNullPipeline(t *testing.T) {
	transform1 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "color",
		},
		Source:   "source",
		Pipeline: []Transformation{},
	}
	r := &inject.Robot{}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}
	_, err = newTransformPipeline(context.Background(), source, transform1, r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "pipeline has no transforms")

	transform2 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "color",
		},
		Source:   "source",
		Pipeline: []Transformation{{Type: "identity", Attributes: nil}},
	}
	pipe, err := newTransformPipeline(context.Background(), source, transform2, r)
	test.That(t, err, test.ShouldBeNil)
	outImg, _, err := pipe.Next(context.Background()) // should not transform anything
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 720)
}

func TestPipeIntoPipe(t *testing.T) {
	r := &inject.Robot{}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}

	intrinsics1 := &transform.PinholeCameraIntrinsics{Width: 1280, Height: 720}
	transform1 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream:           "color",
			CameraParameters: intrinsics1,
		},
		Source:   "source",
		Pipeline: []Transformation{{Type: "rotate", Attributes: config.AttributeMap{}}},
	}
	intrinsics2 := &transform.PinholeCameraIntrinsics{Width: 100, Height: 200}
	transform2 := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream:           "color",
			CameraParameters: intrinsics2,
		},
		Source: "transform2",
		Pipeline: []Transformation{
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}
	transformWrong := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
		Source: "transform2",
		Pipeline: []Transformation{
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}

	pipe1, err := newTransformPipeline(context.Background(), source, transform1, r)
	test.That(t, err, test.ShouldBeNil)
	outImg, _, err := pipe1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 720)
	prop, err := pipe1.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.(*transform.PinholeCameraIntrinsics).Width, test.ShouldEqual, 1280)
	test.That(t, prop.(*transform.PinholeCameraIntrinsics).Height, test.ShouldEqual, 720)
	// transform pipeline into pipeline
	pipe2, err := newTransformPipeline(context.Background(), pipe1, transform2, r)
	test.That(t, err, test.ShouldBeNil)
	outImg, _, err = pipe2.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outImg.Bounds().Dx(), test.ShouldEqual, 100)
	test.That(t, outImg.Bounds().Dy(), test.ShouldEqual, 200)
	test.That(t, err, test.ShouldBeNil)
	prop, err = pipe2.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.(*transform.PinholeCameraIntrinsics).Width, test.ShouldEqual, 100)
	test.That(t, prop.(*transform.PinholeCameraIntrinsics).Height, test.ShouldEqual, 200)
	// should error - color image cannot be depth resized
	pipeWrong, err := newTransformPipeline(context.Background(), pipe1, transformWrong, r)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = pipeWrong.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "don't know how to make image.Gray16")
}
