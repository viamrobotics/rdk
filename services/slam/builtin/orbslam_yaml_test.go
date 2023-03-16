package builtin_test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"gopkg.in/yaml.v2"

	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/slam/builtin"
	slamConfig "go.viam.com/slam/config"
	slamTesthelper "go.viam.com/slam/testhelper"
)

const (
	yamlFilePrefixBytes = "%YAML:1.0\n"
	slamTimeFormat      = "2006-01-02T15:04:05.0000Z"
)

// function to search a SLAM data dir for a .yaml file. returns the timestamp and filepath.
func findLastYAML(folderName string) (string, string, error) {
	root := filepath.Join(folderName, "config")
	yamlExt := ".yaml"
	yamlTimestamp := time.Time{}
	var yamlPath string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if !entry.IsDir() && filepath.Ext(path) == yamlExt {
			// check if the file uses our format and grab timestamp if it does
			timestampLoc := strings.Index(entry.Name(), "_data_") + len("_data_")
			if timestampLoc != -1+len("_data_") {
				timestamp, err := time.Parse(slamTimeFormat, entry.Name()[timestampLoc:strings.Index(entry.Name(), yamlExt)])
				if err != nil {
					return errors.Wrap(err, "Unable to parse yaml")
				}
				if timestamp.After(yamlTimestamp) {
					yamlTimestamp = timestamp
					yamlPath = path
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if yamlTimestamp.IsZero() {
		return "", "", errors.New("No yaml file found")
	}
	return yamlTimestamp.UTC().Format(slamTimeFormat), yamlPath, nil
}

func TestOrbslamYAMLNew(t *testing.T) {
	logger := golog.NewTestLogger(t)
	name, err := slamTesthelper.CreateTempFolderArchitecture(logger)
	test.That(t, err, test.ShouldBeNil)

	createFakeSLAMLibraries()
	useLiveData := true
	dataRateMs := 200
	attrCfgGood := &slamConfig.AttrConfig{
		Sensors: []string{"good_color_camera"},
		ConfigParams: map[string]string{
			"mode":              "mono",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
		},
		DataDirectory: name,
		DataRateMsec:  dataRateMs,
		Port:          "localhost:4445",
		UseLiveData:   &useLiveData,
	}
	attrCfgGoodHighDataRateMsec := &slamConfig.AttrConfig{
		Sensors: []string{"good_color_camera"},
		ConfigParams: map[string]string{
			"mode":              "mono",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
		},
		DataDirectory: name,
		DataRateMsec:  10000,
		Port:          "localhost:4445",
		UseLiveData:   &useLiveData,
	}
	attrCfgBadCam := &slamConfig.AttrConfig{
		Sensors: []string{"bad_camera_intrinsics"},
		ConfigParams: map[string]string{
			"mode":              "mono",
			"orb_n_features":    "1000",
			"orb_scale_factor":  "1.2",
			"orb_n_levels":      "8",
			"orb_n_ini_th_fast": "20",
			"orb_n_min_th_fast": "7",
		},
		DataDirectory: name,
		DataRateMsec:  dataRateMs,
		Port:          "localhost:4445",
		UseLiveData:   &useLiveData,
	}
	var fakeMap string
	var fakeMapTimestamp string
	t.Run("New orbslamv3 service with good camera and defined params", func(t *testing.T) {
		// Create slam service
		grpcServer, port := setupTestGRPCServer(t)
		attrCfgGood.Port = "localhost:" + strconv.Itoa(port)

		svc, err := createSLAMService(t, attrCfgGood, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

		yamlFileTimeStampGood, yamlFilePathGood, err := findLastYAML(name)

		fakeMapTimestamp = yamlFileTimeStampGood
		test.That(t, err, test.ShouldBeNil)

		yamlDataAll, err := os.ReadFile(yamlFilePathGood)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, yamlDataAll[:len(yamlFilePrefixBytes)], test.ShouldResemble, []byte(yamlFilePrefixBytes))

		yamlData := bytes.Replace(yamlDataAll, []byte(yamlFilePrefixBytes), []byte(""), 1)
		orbslam := builtin.ORBsettings{}
		err = yaml.Unmarshal(yamlData, &orbslam)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, orbslam.Width, test.ShouldEqual, 1280)
		test.That(t, orbslam.NLevels, test.ShouldEqual, 8)
		test.That(t, orbslam.ScaleFactor, test.ShouldEqual, 1.2)
		test.That(t, orbslam.LoadMapLoc, test.ShouldEqual, "")
		test.That(t, orbslam.FPSCamera, test.ShouldEqual, 5)

		//save a fake map for the next map using the previous timestamp
		fakeMap = filepath.Join(name, "map", attrCfgGood.Sensors[0]+"_data_"+yamlFileTimeStampGood)
		outfile, err := os.Create(fakeMap + ".osa")
		test.That(t, err, test.ShouldBeNil)
		err = outfile.Close()
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("New orbslamv3 service with previous map and good camera", func(t *testing.T) {
		// Create slam service
		grpcServer, port := setupTestGRPCServer(t)
		attrCfgGood.Port = "localhost:" + strconv.Itoa(port)

		svc, err := createSLAMService(t, attrCfgGood, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

		// Should have the same name due to map being found
		yamlFileTimeStampGood, yamlFilePathGood, err := findLastYAML(name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, yamlFileTimeStampGood, test.ShouldEqual, fakeMapTimestamp)

		// check if map was specified to load
		yamlDataAll, err := os.ReadFile(yamlFilePathGood)
		test.That(t, err, test.ShouldBeNil)
		yamlData := bytes.Replace(yamlDataAll, []byte(yamlFilePrefixBytes), []byte(""), 1)
		orbslam := builtin.ORBsettings{}
		err = yaml.Unmarshal(yamlData, &orbslam)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, orbslam.LoadMapLoc, test.ShouldEqual, "\""+fakeMap+"\"")
	})

	t.Run("New orbslamv3 service with high dataRateMs", func(t *testing.T) {
		// Create slam service
		grpcServer, port := setupTestGRPCServer(t)
		attrCfgGoodHighDataRateMsec.Port = "localhost:" + strconv.Itoa(port)

		svc, err := createSLAMService(t, attrCfgGoodHighDataRateMsec, "fake_orbslamv3", logger, false, true)
		test.That(t, err, test.ShouldBeNil)

		grpcServer.Stop()
		test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

		yamlFileTimeStampGood, yamlFilePathGood, err := findLastYAML(name)

		fakeMapTimestamp = yamlFileTimeStampGood
		test.That(t, err, test.ShouldBeNil)

		yamlDataAll, err := os.ReadFile(yamlFilePathGood)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, yamlDataAll[:len(yamlFilePrefixBytes)], test.ShouldResemble, []byte(yamlFilePrefixBytes))

		yamlData := bytes.Replace(yamlDataAll, []byte(yamlFilePrefixBytes), []byte(""), 1)
		orbslam := builtin.ORBsettings{}
		err = yaml.Unmarshal(yamlData, &orbslam)
		// Even though the real fps is 0.1 Hz, we set it to 1
		test.That(t, orbslam.FPSCamera, test.ShouldEqual, 1)
	})

	t.Run("New orbslamv3 service with camera that errors from bad intrinsics", func(t *testing.T) {
		// Create slam service
		_, err := createSLAMService(t, attrCfgBadCam, "fake_orbslamv3", logger, false, false)

		test.That(t, err.Error(), test.ShouldContainSubstring,
			transform.NewNoIntrinsicsError(fmt.Sprintf("Invalid size (%#v, %#v)", 0, 0)).Error())
	})

	t.Run("New orbslamv3 service with camera that errors from bad orbslam params", func(t *testing.T) {
		// check if a param is empty
		attrCfgBadParam1 := &slamConfig.AttrConfig{
			Sensors: []string{"good_color_camera"},
			ConfigParams: map[string]string{
				"mode":              "mono",
				"orb_n_features":    "",
				"orb_scale_factor":  "1.2",
				"orb_n_levels":      "8",
				"orb_n_ini_th_fast": "20",
				"orb_n_min_th_fast": "7",
			},
			DataDirectory: name,
			DataRateMsec:  dataRateMs,
			Port:          "localhost:4445",
			UseLiveData:   &useLiveData,
		}
		// Create slam service
		_, err := createSLAMService(t, attrCfgBadParam1, "fake_orbslamv3", logger, false, false)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Parameter orb_n_features has an invalid definition")

		attrCfgBadParam2 := &slamConfig.AttrConfig{
			Sensors: []string{"good_color_camera"},
			ConfigParams: map[string]string{
				"mode":              "mono",
				"orb_n_features":    "1000",
				"orb_scale_factor":  "afhaf",
				"orb_n_levels":      "8",
				"orb_n_ini_th_fast": "20",
				"orb_n_min_th_fast": "7",
			},
			DataDirectory: name,
			DataRateMsec:  dataRateMs,
			Port:          "localhost:4445",
			UseLiveData:   &useLiveData,
		}
		_, err = createSLAMService(t, attrCfgBadParam2, "fake_orbslamv3", logger, false, false)

		test.That(t, err.Error(), test.ShouldContainSubstring, "Parameter orb_scale_factor has an invalid definition")
	})

	closeOutSLAMService(t, name)
}
