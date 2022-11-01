package builtin_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	_ "go.viam.com/rdk/components/camera/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func TestGetDetectorNames(t *testing.T) {
	srv, r := createService(t, "../data/fake.json")
	names, err := srv.DetectorNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	t.Logf("names %v", names)
	test.That(t, names, test.ShouldContain, "detector_3")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestDetections(t *testing.T) {
	r, err := buildRobotWithFakeCamera(t)
	test.That(t, err, test.ShouldBeNil)
	srv, err := vision.FirstFromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	dets, err := srv.DetectionsFromCamera(context.Background(), "fake_cam", "detect_red", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dets, test.ShouldHaveLength, 1)
	test.That(t, dets[0].Label(), test.ShouldEqual, "red")
	test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
	box := dets[0].BoundingBox()
	test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
	test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
	// errors
	_, err = srv.DetectionsFromCamera(context.Background(), "fake_cam", "detect_blue", map[string]interface{}{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such vision model with name")
	_, err = srv.DetectionsFromCamera(context.Background(), "real_cam", "detect_red", map[string]interface{}{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"rdk:component:camera/real_cam\" not found")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestAddRemoveDetector(t *testing.T) {
	srv, r := createService(t, "../data/empty.json")
	// success
	cfg := vision.VisModelConfig{
		Name: "test",
		Type: "color_detector",
		Parameters: config.AttributeMap{
			"detect_color":      "#112233",
			"hue_tolerance_pct": 0.4,
			"value_cutoff_pct":  0.2,
			"segment_size_px":   100,
		},
	}
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg2 := vision.VisModelConfig{
		Name: "testdetector", Type: "tflite_detector",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}

	err := srv.AddDetector(context.Background(), cfg, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	names, err := srv.DetectorNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	// check if associated segmenter was added
	namesSeg, err := srv.SegmenterNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, namesSeg, test.ShouldContain, "test_segmenter")
	// failure
	cfg.Name = "will_fail"
	cfg.Type = "wrong_type"
	err = srv.AddDetector(context.Background(), cfg, map[string]interface{}{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	names, err = srv.DetectorNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	test.That(t, names, test.ShouldNotContain, "will_fail")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	// test new Detections directly on image
	err = srv.AddDetector(context.Background(), cfg2, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	img, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	dets, err := srv.Detections(context.Background(), img, "testdetector", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dets, test.ShouldNotBeNil)
	test.That(t, dets[0].Label(), test.ShouldResemble, "17")
	test.That(t, dets[0].Score(), test.ShouldBeGreaterThan, 0.78)
	// remove detector
	err = srv.RemoveDetector(context.Background(), "test", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	names, err = srv.DetectorNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldNotContain, "test")
	// check if associated segmenter was removed
	namesSeg, err = srv.SegmenterNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, namesSeg, test.ShouldNotContain, "test_segmenter")
}
