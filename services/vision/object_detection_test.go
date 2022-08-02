package vision_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func TestGetDetectorNames(t *testing.T) {
	srv, r := createService(t, "data/fake.json")
	names, err := srv.GetDetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	t.Logf("names %v", names)
	test.That(t, names, test.ShouldContain, "detector_3")
	// check that segmenter was added too
	segNames, err := srv.GetSegmenterNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segNames, test.ShouldContain, "detector_3")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestGetDetections(t *testing.T) {
	r := buildRobotWithFakeCamera(t)
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	dets, err := srv.GetDetectionsFromCamera(context.Background(), "fake_cam", "detect_red")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dets, test.ShouldHaveLength, 1)
	test.That(t, dets[0].Label(), test.ShouldEqual, "red")
	test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
	box := dets[0].BoundingBox()
	test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
	test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
	// errors
	_, err = srv.GetDetectionsFromCamera(context.Background(), "fake_cam", "detect_blue")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	_, err = srv.GetDetectionsFromCamera(context.Background(), "real_cam", "detect_red")
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"rdk:component:camera/real_cam\" not found")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestAddDetector(t *testing.T) {
	srv, r := createService(t, "data/empty.json")
	// success
	cfg := vision.DetectorConfig{
		Name: "test",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg2 := vision.DetectorConfig{
		Name: "testdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}

	err := srv.AddDetector(context.Background(), cfg)
	test.That(t, err, test.ShouldBeNil)
	names, err := srv.GetDetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	// test that segmenter was also added
	segNames, err := srv.GetSegmenterNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segNames, test.ShouldContain, "test")
	// failure
	cfg.Name = "will_fail"
	cfg.Type = "wrong_type"
	err = srv.AddDetector(context.Background(), cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	names, err = srv.GetDetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	test.That(t, names, test.ShouldNotContain, "will_fail")
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	// test new GetDetections directly on image
	err = srv.AddDetector(context.Background(), cfg2)
	test.That(t, err, test.ShouldBeNil)
	img, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	dets, err := srv.GetDetections(context.Background(), img, "testdetector")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dets, test.ShouldNotBeNil)
	test.That(t, dets[0].Label(), test.ShouldResemble, "17")
	test.That(t, dets[0].Score(), test.ShouldBeGreaterThan, 0.79)
	box := dets[0].BoundingBox()
	test.That(t, box.Min, test.ShouldResemble, image.Point{126, 42})
	test.That(t, box.Max, test.ShouldResemble, image.Point{199, 162})
}
