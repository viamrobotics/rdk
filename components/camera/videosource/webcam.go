package videosource

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/driver"
	mediadevicescamera "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
)

const model = "webcam"

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		model,
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return NewWebcamSource(ctx, attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf WebcamAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		}, &WebcamAttrs{})

	registry.RegisterDiscoveryFunction(
		discovery.NewQuery(camera.SubtypeName, model),
		func(ctx context.Context) (interface{}, error) { return Discover(ctx, getVideoDrivers) },
	)
}

func getVideoDrivers() []driver.Driver {
	return driver.GetManager().Query(driver.FilterVideoRecorder())
}

// CameraConfig is collection of configuration options for a camera.
type CameraConfig struct {
	Label      string
	Status     driver.State
	Properties []prop.Media
}

// Discover webcam attributes.
func Discover(ctx context.Context, getDrivers func() []driver.Driver) (*pb.Webcams, error) {
	var webcams []*pb.Webcam
	drivers := getDrivers()
	for _, d := range drivers {
		driverInfo := d.Info()

		props, err := getProperties(d)
		if len(props) == 0 {
			rlog.Logger.Debugw("no properties detected for driver, skipping discovery...", "driver", driverInfo.Label)
			continue
		} else if err != nil {
			rlog.Logger.Debugw("cannot access driver properties, skipping discovery...", "driver", driverInfo.Label, "error", err)
			continue
		}

		if d.Status() == driver.StateRunning {
			rlog.Logger.Debugw("driver is in use, skipping discovery...", "driver", driverInfo.Label)
			continue
		}

		labelParts := strings.Split(driverInfo.Label, mediadevicescamera.LabelSeparator)
		label := labelParts[len(labelParts)-1]
		wc := &pb.Webcam{
			Label:      label,
			Status:     string(d.Status()),
			Properties: make([]*pb.Property, 0, len(d.Properties())),
		}

		for _, prop := range props {
			pbProp := &pb.Property{
				Video: &pb.Video{
					Width:       int32(prop.Video.Width),
					Height:      int32(prop.Video.Height),
					FrameFormat: string(prop.Video.FrameFormat),
				},
			}
			wc.Properties = append(wc.Properties, pbProp)
		}
		webcams = append(webcams, wc)
	}
	return &pb.Webcams{Webcams: webcams}, nil
}

func getProperties(d driver.Driver) (_ []prop.Media, err error) {
	// Need to open driver to get properties
	if d.Status() == driver.StateClosed {
		errOpen := d.Open()
		if errOpen != nil {
			return nil, errOpen
		}
		defer func() {
			if errClose := d.Close(); errClose != nil {
				err = errClose
			}
		}()
	}
	return d.Properties(), err
}

// WebcamAttrs is the attribute struct for webcams.
type WebcamAttrs struct {
	*camera.AttrConfig
	Format      string `json:"format"`
	Path        string `json:"path"`
	PathPattern string `json:"path_pattern"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func makeConstraints(attrs *WebcamAttrs, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {
	minWidth := 0
	maxWidth := 4096
	idealWidth := 800
	minHeight := 0
	maxHeight := 2160
	idealHeight := 600

	if attrs.Width > 0 {
		minWidth = 0
		maxWidth = attrs.Width
		idealWidth = attrs.Width
	}

	if attrs.Height > 0 {
		minHeight = 0
		maxHeight = attrs.Height
		idealHeight = attrs.Height
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWidth}
			constraint.Height = prop.IntRanged{minHeight, maxHeight, idealHeight}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			if attrs.Format == "" {
				constraint.FrameFormat = prop.FrameFormatOneOf{
					frame.FormatI420,
					frame.FormatI444,
					frame.FormatYUY2,
					frame.FormatUYVY,
					frame.FormatRGBA,
					frame.FormatMJPEG,
					frame.FormatNV12,
					frame.FormatNV21,
				}
			} else {
				constraint.FrameFormat = prop.FrameFormatExact(attrs.Format)
			}

			if debug {
				logger.Debugf("constraints: %v", constraint)
			}
		},
	}
}

// NewWebcamSource returns a new source based on a webcam discovered from the given attributes.
func NewWebcamSource(ctx context.Context, attrs *WebcamAttrs, logger golog.Logger) (camera.Camera, error) {
	var err error

	debug := attrs.Debug

	constraints := makeConstraints(attrs, debug, logger)

	if attrs.Path != "" {
		return tryWebcamOpen(ctx, attrs, attrs.Path, constraints)
	}

	var pattern *regexp.Regexp
	if attrs.PathPattern != "" {
		pattern, err = regexp.Compile(attrs.PathPattern)
		if err != nil {
			return nil, err
		}
	}

	labels := gostream.QueryVideoDeviceLabels()
	for _, label := range labels {
		if debug {
			logger.Debugf("%s", label)
		}

		if pattern != nil && !pattern.MatchString(label) {
			if debug {
				logger.Debug("\t skipping because of pattern")
			}
			continue
		}

		s, err := tryWebcamOpen(ctx, attrs, label, constraints)
		if err == nil {
			if debug {
				logger.Debug("\t USING")
			}

			return s, nil
		}
		if debug {
			logger.Debugf("\t %w", err)
		}
	}

	return nil, errors.New("found no webcams")
}

func tryWebcamOpen(ctx context.Context,
	attrs *WebcamAttrs,
	path string,
	constraints mediadevices.MediaStreamConstraints,
) (camera.Camera, error) {
	source, err := gostream.GetNamedVideoSource(filepath.Base(path), constraints)
	if err != nil {
		return nil, err
	}
	var intrinsics *transform.PinholeCameraIntrinsics
	if attrs.AttrConfig != nil {
		intrinsics = attrs.AttrConfig.CameraParameters
	}
	return camera.NewFromSource(ctx, source, intrinsics, camera.StreamType(attrs.Stream))
}
