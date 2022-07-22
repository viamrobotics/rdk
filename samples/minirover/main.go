// Package main is the work-in-progress robotic land rover from Viam.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/perf"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/vision/segmentation"
)

// TODO.
const (
	PanCenter  = 94
	TiltCenter = 113
)

var logger = golog.NewDevelopmentLogger("minirover")

func findTopInSegment(img *segmentation.SegmentedImage, segment int) image.Point {
	mid := img.Width() / 2
	for y := 0; y < img.Height(); y++ {
		for x := mid; x < img.Width(); x++ {
			p := image.Point{x, y}
			s := img.GetSegment(p)
			if s == segment {
				return p
			}

			p = image.Point{mid - (x - mid), y}
			s = img.GetSegment(p)
			if s == segment {
				return p
			}
		}
	}
	return image.Point{0, 0}
}

func findBlack(ctx context.Context, img *rimage.Image, logger golog.Logger) (image.Point, image.Image, error) {
	_, span := trace.StartSpan(ctx, "findBlack")
	defer span.End()

	for x := 1; x < img.Width(); x += 3 {
		for y := 1; y < img.Height(); y += 3 {
			c := img.GetXY(x, y)
			if c.Distance(rimage.Black) > 1 {
				continue
			}

			x, err := segmentation.ShapeWalk(img, nil,
				image.Point{x, y},
				segmentation.ShapeWalkOptions{
					SkipCleaning: true,
					// Debug: true,
				},
				logger,
			)
			if err != nil {
				return image.Point{}, nil, err
			}

			if x.PixelsInSegmemnt(1) > 10000 {
				return findTopInSegment(x, 1), x, nil
			}
		}
	}

	return image.Point{}, nil, errors.New("no black found")
}

// Rover TODO.
type Rover struct {
	pan, tilt servo.Servo
}

func (r *Rover) neckCenter(ctx context.Context) error {
	return r.neckPosition(ctx, PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(ctx context.Context, left int) error {
	return r.neckPosition(ctx, uint8(PanCenter+(left*-30)), uint8(TiltCenter-20))
}

func (r *Rover) neckPosition(ctx context.Context, pan, tilt uint8) error {
	logger.Debugf("neckPosition to %v %v", pan, tilt)
	return multierr.Combine(r.pan.Move(ctx, pan), r.tilt.Move(ctx, tilt))
}

// Ready TODO.
func (r *Rover) Ready(ctx context.Context, theRobot robot.Robot) error {
	logger.Debug("minirover2 Ready called")
	cam, err := camera.FromRobot(theRobot, "front")
	if err != nil {
		return err
	}

	// doing this in a goroutine so i can see camera and servo data in web ui, but probably not right long term
	if false {
		utils.PanicCapturingGo(func() {
			for {
				if !utils.SelectContextOrWait(ctx, time.Second) {
					return
				}
				var depthErr bool
				func() {
					img, release, err := cam.Next(ctx)
					if err != nil {
						logger.Debugf("error from camera %s", err)
						return
					}
					defer release()
					pc := rimage.ConvertToImageWithDepth(img)
					if pc.Depth == nil {
						logger.Warn("no depth data")
						depthErr = true
						return
					}
					err = pc.WriteTo(artifact.MustNewPath(fmt.Sprintf("samples/minirover/rover-centering-%d.both.gz", time.Now().Unix())))
					if err != nil {
						logger.Debugf("error writing %s", err)
					}
				}()
				if depthErr {
					return
				}
			}
		})
	}

	return nil
}

// NewRover TODO.
func NewRover(ctx context.Context, r robot.Robot) (*Rover, error) {
	rover := &Rover{}
	var err error
	rover.pan, err = servo.FromRobot(r, "pan")
	if err != nil {
		return nil, err
	}
	rover.tilt, err = servo.FromRobot(r, "tilt")
	if err != nil {
		return nil, err
	}

	if false {
		utils.PanicCapturingGo(func() {
			for {
				if !utils.SelectContextOrWait(ctx, 1500*time.Millisecond) {
					return
				}
				err := rover.neckCenter(ctx)
				if err != nil {
					panic(err)
				}

				if !utils.SelectContextOrWait(ctx, 1500*time.Millisecond) {
					return
				}

				err = rover.neckOffset(ctx, -1)
				if err != nil {
					panic(err)
				}

				if !utils.SelectContextOrWait(ctx, 1500*time.Millisecond) {
					return
				}

				err = rover.neckOffset(ctx, 1)
				if err != nil {
					panic(err)
				}
			}
		})
	} else {
		err := rover.neckCenter(ctx)
		if err != nil {
			return nil, err
		}
	}

	logger.Debug("rover ready")

	return rover, nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	exp := perf.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	cfg, err := config.Read(ctx, "samples/minirover/config.json", logger)
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	rover, err := NewRover(ctx, myRobot)
	if err != nil {
		return err
	}
	err = rover.Ready(ctx, myRobot)
	if err != nil {
		return err
	}

	options, err := weboptions.FromConfig(cfg)
	if err != nil {
		return err
	}
	options.Pprof = true
	return web.RunWeb(ctx, myRobot, options, logger)
}
