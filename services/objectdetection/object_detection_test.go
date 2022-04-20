package objectdetection_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/objectdetection"
)

func createService(t *testing.T, filePath string) objectdetection.Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), filePath, logger)
	test.That(t, err, test.ShouldBeNil)
	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	return srv
}

func TestDetectorNames(t *testing.T) {
	srv := createService(t, "data/fake.json")
	names, err := srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "detector_3")
}

func TestAddDetector(t *testing.T) {
	srv := createService(t, "data/empty.json")
	// success
	cfg := objectdetection.Config{
		Name: "test",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	ok, err := srv.AddDetector(context.Background(), cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	names, err := srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	// failure
	cfg.Name = "will_fail"
	cfg.Type = "wrong_type"
	ok, err = srv.AddDetector(context.Background(), cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, ok, test.ShouldBeFalse)
	names, err = srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	test.That(t, names, test.ShouldNotContain, "will_fail")
}
