package slam_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam"
)

func TestOrbslamYAMLNew(t *testing.T) {
	name, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()
	dataRateMs := 100
	attrCfgGood := &slam.AttrConfig{
		Algorithm: "fake_orbslamv3",
		Sensors:   []string{"good_camera"},
		ConfigParams: map[string]string{
			"mode":              "mono",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
		},
		DataDirectory: name,
		DataRateMs:    dataRateMs,
		Port:          "localhost:4445",
	}
	attrCfgBadCam := &slam.AttrConfig{
		Algorithm: "fake_orbslamv3",
		Sensors:   []string{"bad_camera_intrinsics"},
		ConfigParams: map[string]string{
			"mode":              "mono",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
		},
		DataDirectory: name,
		DataRateMs:    dataRateMs,
		Port:          "localhost:4445",
	}
	t.Run("New orbslamv3 service with good camera and defined params", func(t *testing.T) {
		// Create slam service
		logger := golog.NewTestLogger(t)
		grpcServer := setupTestGRPCServer(attrCfgGood.Port)
		svc, err := createSLAMService(t, attrCfgGood, logger, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service with camera that errors from bad intrinsics", func(t *testing.T) {
		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfgBadCam, logger, false)

		test.That(t, err.Error(), test.ShouldContainSubstring,
			transform.NewNoIntrinsicsError(fmt.Sprintf("Invalid size (%#v, %#v)", 0, 0)).Error())
	})

	t.Run("New orbslamv3 service with camera that errors from bad orbslam params", func(t *testing.T) {
		// check if a param is empty
		attrCfgBadParam1 := &slam.AttrConfig{
			Algorithm: "fake_orbslamv3",
			Sensors:   []string{"good_camera"},
			ConfigParams: map[string]string{
				"mode":              "mono",
				"orb_n_features":    "",
				"orb_scale_factor":  "1.2",
				"orb_n_levels":      "8",
				"orb_n_ini_th_fast": "20",
				"orb_n_min_th_fast": "7",
			},
			DataDirectory: name,
			DataRateMs:    dataRateMs,
			Port:          "localhost:4445",
		}
		// Create slam service
		logger := golog.NewTestLogger(t)
		_, err := createSLAMService(t, attrCfgBadParam1, logger, false)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Parameter orb_n_features has an invalid definition")

		attrCfgBadParam2 := &slam.AttrConfig{
			Algorithm: "fake_orbslamv3",
			Sensors:   []string{"good_camera"},
			ConfigParams: map[string]string{
				"mode":              "mono",
				"orb_n_features":    "1000",
				"orb_scale_factor":  "afhaf",
				"orb_n_levels":      "8",
				"orb_n_ini_th_fast": "20",
				"orb_n_min_th_fast": "7",
			},
			DataDirectory: name,
			DataRateMs:    dataRateMs,
			Port:          "localhost:4445",
		}
		_, err = createSLAMService(t, attrCfgBadParam2, logger, false)

		test.That(t, err.Error(), test.ShouldContainSubstring, "Parameter orb_scale_factor has an invalid definition")
	})

	closeOutSLAMService(t, name)
}
