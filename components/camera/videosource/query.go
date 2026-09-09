package videosource

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// minResolutionDimension is set to 2 to ensure proper fitness distance calculation for resolution selection.
// Setting this to 0 would cause mediadevices' IntRanged.Compare() method to treat all values smaller than ideal
// as equally acceptable. See https://github.com/pion/mediadevices/blob/c10fb000dbbb28597e068468f3175dc68a281bfd/pkg/prop/int.go#L104
// Setting it to 1 could theoretically allow 1x1 resolutions. 2 is small enough and even,
// allowing all real camera resolutions while ensuring proper distance calculations.
const minResolutionDimension = 2

// Below is adapted from github.com/pion/mediadevices.
// It is further adapted from gostream's query.go
// However, this is the minimum code needed for webcam to work, placed in this directory.
// This vastly improves the debugging and feature development experience, by not over-DRY-ing.

// makeConstraints is a helper that returns constraints to mediadevices in order to find and make a video source.
// Constraints are specifications for the video stream such as frame format, resolution etc.
func makeConstraints(conf *WebcamConfig, logger logging.Logger) mediadevices.MediaStreamConstraints {
	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			if conf.Width > 0 {
				constraint.Width = prop.IntExact(conf.Width)
			} else {
				constraint.Width = prop.IntRanged{Min: minResolutionDimension, Ideal: 640, Max: 4096}
			}

			if conf.Height > 0 {
				constraint.Height = prop.IntExact(conf.Height)
			} else {
				constraint.Height = prop.IntRanged{Min: minResolutionDimension, Ideal: 480, Max: 2160}
			}

			if conf.FrameRate > 0.0 {
				constraint.FrameRate = prop.FloatExact(conf.FrameRate)
			} else {
				constraint.FrameRate = prop.FloatRanged{Min: 0.0, Ideal: 30.0, Max: 140.0}
			}

			if conf.Format == "" {
				constraint.FrameFormat = prop.FrameFormatOneOf{
					frame.FormatI420,
					frame.FormatI444,
					frame.FormatYUY2,
					frame.FormatUYVY,
					frame.FormatRGBA,
					frame.FormatMJPEG,
					frame.FormatNV12,
					frame.FormatNV21,
					frame.FormatZ16,
				}
			} else {
				constraint.FrameFormat = prop.FrameFormatExact(conf.Format)
			}

			logger.Debugf("constraints: %v", constraint)
		},
	}
}

// findReaderAndDriver finds a video device and returns an image reader and the driver instance,
// as well as the path to the driver.
func findReaderAndDriver(
	conf *WebcamConfig,
	path string,
	logger logging.Logger,
) (video.Reader, driver.Driver, string, error) {
	if runtime.GOOS == "linux" {
		// TODO(RSDK-12789): Separate discover() calls from Initialize() calls.
		// So we can call Initialize() only once, and call discover() as many times as we need.
		mediadevicescamera.Initialize()
	}

	constraints := makeConstraints(conf, logger)

	// Handle specific path
	if path != "" {
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err == nil {
			path = resolvedPath
		}

		var searchPath string
		if runtime.GOOS == "windows" {
			// Use full path for windows driver paths for compatibility
			searchPath = path
		} else {
			searchPath = filepath.Base(path)
		}

		reader, driver, err := getReaderAndDriver(labelFilter(searchPath, true, false), searchPath, constraints, logger)
		if err != nil {
			return nil, nil, "", err
		}
		return reader, driver, path, nil
	}

	// Handle "any" path
	reader, driver, err := getReaderAndDriver(nil, "", constraints, logger)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "found no webcams")
	}
	labels := strings.Split(driver.Info().Label, mediadevicescamera.LabelSeparator)
	if len(labels) == 0 {
		logger.Error("no labels parsed from driver")
		return nil, nil, "", errors.New("no labels parsed from driver")
	}
	path = labels[0] // path is always the first element

	return reader, driver, path, nil
}

// findReaderAndDriverByName finds a video device whose driver Name matches the given name. The driver Name is the OS-reported device name
func findReaderAndDriverByName(
	conf *WebcamConfig,
	name string,
	logger logging.Logger,
) (video.Reader, driver.Driver, string, error) {
	constraints := makeConstraints(conf, logger)

	reader, driver, err := getReaderAndDriver(labelFilter(name, false, true), name, constraints, logger)
	if err != nil {
		return nil, nil, "", err
	}
	labels := strings.Split(driver.Info().Label, mediadevicescamera.LabelSeparator)
	return reader, driver, labels[0], nil
}

// getReaderAndDriver attempts to find a device (not a screen) matching the given filter.
// If filter is nil, it finds any device. target is the identifier being searched for and is only used in
// error messages.
func getReaderAndDriver(
	filter driver.FilterFn,
	target string,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (video.Reader, driver.Driver, error) {
	d, selectedMedia, err := getUserVideoDriver(constraints, filter, target, logger)
	if err != nil {
		return nil, nil, err
	}

	if err := openDriver(d); err != nil {
		return nil, nil, err
	}

	success := false
	defer func() {
		if !success {
			if err := d.Close(); err != nil {
				logger.Errorw("failed to close driver after error", "error", err)
			}
		}
	}()

	reader, err := newReaderFromDriver(d, selectedMedia)
	if err != nil {
		return nil, nil, err
	}

	success = true // signal success to the deferred func
	return reader, d, nil
}

func getUserVideoDriver(
	constraints mediadevices.MediaStreamConstraints,
	filter driver.FilterFn,
	target string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectVideo(videoConstraints, filter, target, logger)
}

func openDriver(d driver.Driver) error {
	if ok, err := driver.IsAvailable(d); !errors.Is(err, availability.ErrUnimplemented) && !ok {
		return errors.Wrap(err, "video driver not available")
	}
	if driverStatus := d.Status(); driverStatus != driver.StateClosed {
		return errors.New("video driver in use")
	}
	if err := d.Open(); err != nil {
		return errors.Wrap(err, "cannot open video driver")
	}
	return nil
}

func newReaderFromDriver(
	videoDriver driver.Driver,
	mediaProp prop.Media,
) (video.Reader, error) {
	recorder, ok := videoDriver.(driver.VideoRecorder)
	if !ok {
		return nil, errors.New("driver not a driver.VideoRecorder")
	}
	mediaProp.DiscardFramesOlderThan = time.Second
	return recorder.VideoRecord(mediaProp)
}

// labelFilter matches drivers whose Label equals target, or whose Name equals target when isName is set.
func labelFilter(target string, useSep, isName bool) driver.FilterFn {
	return driver.FilterFn(func(d driver.Driver) bool {
		value := d.Info().Label
		if isName {
			value = d.Info().Name
		}
		if !useSep {
			return value == target
		}
		labels := strings.Split(value, mediadevicescamera.LabelSeparator)
		for _, label := range labels {
			if label == target {
				return true
			}
		}
		return false
	})
}

func selectVideo(
	constraints mediadevices.MediaTrackConstraints,
	filter driver.FilterFn,
	target string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getVideoFilterBase(), getVideoFilter(filter), target, constraints, logger)
}

func getVideoFilterBase() driver.FilterFn {
	typeFilter := driver.FilterVideoRecorder()
	notScreenFilter := driver.FilterNot(driver.FilterDeviceType(driver.Screen))
	return driver.FilterAnd(typeFilter, notScreenFilter)
}

// getVideoFilter combines the base video filter with an optional device-specific filter.
func getVideoFilter(specific driver.FilterFn) driver.FilterFn {
	filter := getVideoFilterBase()
	if specific != nil {
		filter = driver.FilterAnd(filter, specific)
	}
	return filter
}

// select implements SelectSettings algorithm.
// Reference: https://w3c.github.io/mediacapture-main/#dfn-selectsettings
func selectBestDriver(
	baseFilter driver.FilterFn,
	filter driver.FilterFn,
	label string,
	constraints mediadevices.MediaTrackConstraints,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var bestDriver driver.Driver
	var bestProp prop.Media
	minFitnessDist := math.Inf(1)

	baseDrivers := driver.GetManager().Query(baseFilter)
	logger.Debugw("before specific filter, we found the following drivers", "count", len(baseDrivers))
	for i, d := range baseDrivers {
		props := d.Properties()
		logger.Debugw("base driver found",
			"driver_number", fmt.Sprintf("%d/%d", i+1, len(baseDrivers)),
			"label", d.Info().Label,
			"priority", float32(d.Info().Priority),
			"type", d.Info().DeviceType,
			"properties", props)
	}

	driverProperties := queryDriverProperties(filter, logger)
	if len(driverProperties) == 0 {
		msg := fmt.Sprintf("no queryable drivers for video path: '%s'", label)
		if label != "" {
			msg += "; check if the device is available or already in use (busy)"
		}
		return nil, prop.Media{}, errors.New(msg)
	}

	logger.Debugw("found drivers matching specific filter", "count", len(driverProperties))
	for d, props := range driverProperties {
		priority := float64(d.Info().Priority)
		logger.Debugw(
			"considering driver",
			"label", d.Info().Label,
			"priority", priority)
		for _, p := range props {
			fitnessDist, ok := constraints.MediaConstraints.FitnessDistance(p)
			if !ok {
				logger.Debugw("driver does not satisfy any constraints", "label", d.Info().Label)
				continue
			}
			fitnessDistWithPriority := fitnessDist - priority
			logger.Debugw(
				"driver properties satisfy some constraints",
				"label", d.Info().Label,
				"props", p,
				"distance", fitnessDist,
				"distance_with_priority", fitnessDistWithPriority)
			if fitnessDistWithPriority < minFitnessDist {
				minFitnessDist = fitnessDistWithPriority
				bestDriver = d
				bestProp = p
			}
		}
	}

	if bestDriver == nil {
		labels := make([]string, 0, len(driverProperties))
		for d := range driverProperties {
			labels = append(labels, d.Info().Label)
		}
		return nil, prop.Media{}, errors.Errorf(
			"failed to find a queryable driver that matches the config constraints. "+
				"You can try tweaking or relaxing the constraints, e.g. removing or changing the height/width, "+
				"frame format, etc. "+
				"Use the find-webcams discovery service to find valid constraints for your device. "+
				"Devices tried: %s",
			strings.Join(labels, ", "))
	}

	logger.Debugw("winning driver", "label", bestDriver.Info().Label, "props", bestProp)
	selectedMedia := prop.Media{}
	selectedMedia.MergeConstraints(constraints.MediaConstraints)
	selectedMedia.Merge(bestProp)
	return bestDriver, selectedMedia, nil
}

func queryDriverProperties(
	filter driver.FilterFn,
	logger logging.Logger,
) map[driver.Driver][]prop.Media {
	var needToClose []driver.Driver
	drivers := driver.GetManager().Query(filter)
	m := make(map[driver.Driver][]prop.Media)

	for _, d := range drivers {
		var status string
		isAvailable, err := driver.IsAvailable(d)
		if errors.Is(err, availability.ErrUnimplemented) {
			s := d.Status()
			status = string(s)
			isAvailable = s == driver.StateClosed
		} else if err != nil {
			status = err.Error()
		}

		if isAvailable {
			err := d.Open()
			if err != nil {
				logger.Infow("error trying to open driver for querying", "error", err)
				// Skip this driver if we failed to open because we can't get the properties
				continue
			}
			needToClose = append(needToClose, d)
			m[d] = d.Properties()
		} else {
			logger.Infow("driver not available", "name", d.Info().Name, "label", d.Info().Label, "status", status)
		}
	}

	for _, d := range needToClose {
		// Since it was closed, we should close it to avoid a leak
		if err := d.Close(); err != nil {
			logger.Errorw("error closing driver", "error", err)
		}
	}

	return m
}
