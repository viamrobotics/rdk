package gostream

import (
	"image"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/availability"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// below adapted from github.com/pion/mediadevices

// ErrNotFound happens when there is no driver found in a query.
var ErrNotFound = errors.New("failed to find the best driver that fits the constraints")

// DefaultConstraints are suitable for finding any available device.
var DefaultConstraints = mediadevices.MediaStreamConstraints{
	Video: func(constraint *mediadevices.MediaTrackConstraints) {
		constraint.Width = prop.IntRanged{Min: 640, Max: 4096, Ideal: 1920}
		constraint.Height = prop.IntRanged{Min: 400, Max: 2160, Ideal: 1080}
		constraint.FrameRate = prop.FloatRanged{Min: 0, Max: 200, Ideal: 60}
		constraint.FrameFormat = prop.FrameFormatOneOf{
			frame.FormatI420,
			frame.FormatI444,
			frame.FormatYUY2,
			frame.FormatYUYV,
			frame.FormatUYVY,
			frame.FormatRGBA,
			frame.FormatMJPEG,
			frame.FormatNV12,
			frame.FormatZ16,
			frame.FormatNV21, // gives blue tinted image?
		}
	},
}

// GetNamedScreenSource attempts to find a screen device by the given name.
func GetNamedScreenSource(
	name string,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getScreenDriver(constraints, &name, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// GetPatternedScreenSource attempts to find a screen device by the given label pattern.
func GetPatternedScreenSource(
	labelPattern *regexp.Regexp,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getScreenDriverPattern(constraints, labelPattern, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// GetNamedVideoSource attempts to find a video device (not a screen) by the given name.
func GetNamedVideoSource(
	name string,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getUserVideoDriver(constraints, &name, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// GetPatternedVideoSource attempts to find a video device (not a screen) by the given label pattern.
func GetPatternedVideoSource(
	labelPattern *regexp.Regexp,
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getUserVideoDriverPattern(constraints, labelPattern, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// GetAnyScreenSource attempts to find any suitable screen device.
func GetAnyScreenSource(
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getScreenDriver(constraints, nil, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// GetAnyVideoSource attempts to find any suitable video device (not a screen).
func GetAnyVideoSource(
	constraints mediadevices.MediaStreamConstraints,
	logger logging.Logger,
) (MediaSource[image.Image], error) {
	d, selectedMedia, err := getUserVideoDriver(constraints, nil, logger)
	if err != nil {
		return nil, err
	}
	return newVideoSourceFromDriver(d, selectedMedia)
}

// DeviceInfo describes a driver.
type DeviceInfo struct {
	ID         string
	Labels     []string
	Properties []prop.Media
	Priority   driver.Priority
	Error      error
}

// QueryVideoDevices lists all known video devices (not a screen).
func QueryVideoDevices() []DeviceInfo {
	return getDriverInfo(driver.GetManager().Query(getVideoFilterBase()), true)
}

// QueryScreenDevices lists all known screen devices.
func QueryScreenDevices() []DeviceInfo {
	return getDriverInfo(driver.GetManager().Query(getScreenFilterBase()), true)
}

func getDriverInfo(drivers []driver.Driver, useSep bool) []DeviceInfo {
	infos := make([]DeviceInfo, len(drivers))
	for i, d := range drivers {
		if d.Status() == driver.StateClosed {
			if err := d.Open(); err != nil {
				infos[i].Error = err
			} else {
				defer func() {
					infos[i].Error = d.Close()
				}()
			}
		}
		infos[i].ID = d.ID()
		infos[i].Labels = getDriverLabels(d, useSep)
		infos[i].Properties = d.Properties()
		infos[i].Priority = d.Info().Priority
	}
	return infos
}

// QueryScreenDevicesLabels lists all known screen devices.
func QueryScreenDevicesLabels() []string {
	return getDriversLabels(driver.GetManager().Query(getScreenFilterBase()), false)
}

// QueryVideoDeviceLabels lists all known video devices (not a screen).
func QueryVideoDeviceLabels() []string {
	return getDriversLabels(driver.GetManager().Query(getVideoFilterBase()), true)
}

func getDriversLabels(drivers []driver.Driver, useSep bool) []string {
	var labels []string
	for _, d := range drivers {
		labels = append(labels, getDriverLabels(d, useSep)...)
	}
	return labels
}

func getDriverLabels(d driver.Driver, useSep bool) []string {
	if !useSep {
		return []string{d.Info().Label}
	}
	return strings.Split(d.Info().Label, labelSeparator)
}

func getScreenDriver(
	constraints mediadevices.MediaStreamConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectScreen(videoConstraints, label, logger)
}

func getScreenDriverPattern(
	constraints mediadevices.MediaStreamConstraints,
	labelPattern *regexp.Regexp,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectScreenPattern(videoConstraints, labelPattern, logger)
}

func getUserVideoDriver(
	constraints mediadevices.MediaStreamConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectVideo(videoConstraints, label, logger)
}

func getUserVideoDriverPattern(
	constraints mediadevices.MediaStreamConstraints,
	labelPattern *regexp.Regexp,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var videoConstraints mediadevices.MediaTrackConstraints
	if constraints.Video != nil {
		constraints.Video(&videoConstraints)
	}
	return selectVideoPattern(videoConstraints, labelPattern, logger)
}

func newVideoSourceFromDriver(
	videoDriver driver.Driver,
	mediaProp prop.Media,
) (MediaSource[image.Image], error) {
	recorder, ok := videoDriver.(driver.VideoRecorder)
	if !ok {
		return nil, errors.New("driver not a driver.VideoRecorder")
	}

	if ok, err := driver.IsAvailable(videoDriver); !errors.Is(err, availability.ErrUnimplemented) && !ok {
		return nil, errors.Wrap(err, "video driver not available")
	} else if driverStatus := videoDriver.Status(); driverStatus != driver.StateClosed {
		return nil, errors.New("video driver in use")
	} else if err := videoDriver.Open(); err != nil {
		return nil, errors.Wrap(err, "cannot open video driver")
	}

	mediaProp.DiscardFramesOlderThan = time.Second
	reader, err := recorder.VideoRecord(mediaProp)
	if err != nil {
		return nil, err
	}
	return newMediaSource[image.Image](videoDriver, mediaReaderFuncNoCtx[image.Image](reader.Read), mediaProp.Video), nil
}

func labelFilter(target string, useSep bool) driver.FilterFn {
	return driver.FilterFn(func(d driver.Driver) bool {
		if !useSep {
			return d.Info().Label == target
		}
		labels := strings.Split(d.Info().Label, labelSeparator)
		return slices.Contains(labels, target)
	})
}

func labelFilterPattern(labelPattern *regexp.Regexp, useSep bool) driver.FilterFn {
	return driver.FilterFn(func(d driver.Driver) bool {
		if !useSep {
			return labelPattern.MatchString(d.Info().Label)
		}
		labels := strings.Split(d.Info().Label, labelSeparator)
		return slices.ContainsFunc(labels, labelPattern.MatchString)
	})
}

func selectVideo(
	constraints mediadevices.MediaTrackConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getVideoFilterBase(), getVideoFilter(label), constraints, logger)
}

func selectVideoPattern(
	constraints mediadevices.MediaTrackConstraints,
	labelPattern *regexp.Regexp,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getVideoFilterBase(), getVideoFilterPattern(labelPattern), constraints, logger)
}

func selectScreen(
	constraints mediadevices.MediaTrackConstraints,
	label *string,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getScreenFilterBase(), getScreenFilter(label), constraints, logger)
}

func selectScreenPattern(
	constraints mediadevices.MediaTrackConstraints,
	labelPattern *regexp.Regexp,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	return selectBestDriver(getScreenFilterBase(), getScreenFilterPattern(labelPattern), constraints, logger)
}

func getVideoFilterBase() driver.FilterFn {
	typeFilter := driver.FilterVideoRecorder()
	notScreenFilter := driver.FilterNot(driver.FilterDeviceType(driver.Screen))
	return driver.FilterAnd(typeFilter, notScreenFilter)
}

func getVideoFilter(label *string) driver.FilterFn {
	filter := getVideoFilterBase()
	if label != nil {
		filter = driver.FilterAnd(filter, labelFilter(*label, true))
	}
	return filter
}

func getVideoFilterPattern(labelPattern *regexp.Regexp) driver.FilterFn {
	filter := getVideoFilterBase()
	filter = driver.FilterAnd(filter, labelFilterPattern(labelPattern, true))
	return filter
}

func getScreenFilterBase() driver.FilterFn {
	typeFilter := driver.FilterVideoRecorder()
	screenFilter := driver.FilterDeviceType(driver.Screen)
	return driver.FilterAnd(typeFilter, screenFilter)
}

func getScreenFilter(label *string) driver.FilterFn {
	filter := getScreenFilterBase()
	if label != nil {
		filter = driver.FilterAnd(filter, labelFilter(*label, true))
	}
	return filter
}

func getScreenFilterPattern(labelPattern *regexp.Regexp) driver.FilterFn {
	filter := getScreenFilterBase()
	filter = driver.FilterAnd(filter, labelFilterPattern(labelPattern, true))
	return filter
}

// select implements SelectSettings algorithm.
// Reference: https://w3c.github.io/mediacapture-main/#dfn-selectsettings
func selectBestDriver(
	baseFilter driver.FilterFn,
	filter driver.FilterFn,
	constraints mediadevices.MediaTrackConstraints,
	logger logging.Logger,
) (driver.Driver, prop.Media, error) {
	var bestDriver driver.Driver
	var bestProp prop.Media
	minFitnessDist := math.Inf(1)

	baseDrivers := driver.GetManager().Query(baseFilter)
	logger.Debugw("before specific filter, we found the following drivers", "count", len(baseDrivers))
	for _, d := range baseDrivers {
		logger.Debugw(d.Info().Label, "priority", float32(d.Info().Priority), "type", d.Info().DeviceType)
	}

	driverProperties := queryDriverProperties(filter, logger)
	if len(driverProperties) == 0 {
		logger.Debugw("found no drivers matching filter")
	} else {
		logger.Debugw("found drivers matching specific filter", "count", len(driverProperties))
	}
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
		return nil, prop.Media{}, ErrNotFound
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
				logger.Debugw("error opening driver for querying", "error", err)
				// Skip this driver if we failed to open because we can't get the properties
				continue
			}
			needToClose = append(needToClose, d)
			m[d] = d.Properties()
		} else {
			logger.Debugw("driver not available", "name", d.Info().Name, "label", d.Info().Label, "status", status)
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
