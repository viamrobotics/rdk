package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"log"
	"time"

	"github.com/erh/egoutil"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/actions"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/vision/segmentation"

	_ "go.viam.com/robotcore/board/detector"
	_ "go.viam.com/robotcore/rimage/imagesource"

	"go.uber.org/multierr"

	"go.opencensus.io/trace"
)

const (
	PanCenter  = 94
	TiltCenter = 113
)

var logger = golog.NewDevelopmentLogger("minirover")

func init() {
	actions.RegisterAction("dock", func(r api.Robot) {
		err := dock(r)
		if err != nil {
			logger.Errorf("error docking: %s", err)
		}
	})
}

func dock(r api.Robot) error {
	logger.Info("docking started")
	ctx := context.Background()

	cam := r.CameraByName("back")
	if cam == nil {
		return fmt.Errorf("no back camera")
	}

	base := r.BaseByName("pierre")
	if base == nil {
		return fmt.Errorf("no pierre")
	}

	theLidar := r.LidarDeviceByName("lidarOnBase")
	if theLidar == nil {
		return fmt.Errorf("no lidar lidarOnBase")
	}

	for tries := 0; tries < 5; tries++ {

		ms, err := theLidar.Scan(context.Background(), lidar.ScanOptions{})
		if err != nil {
			return err
		}
		back := ms.ClosestToDegree(180)
		logger.Debugf("distance to back: %#v", back)

		if back.Distance() < .55 {
			logger.Info("docking close enough")
			return nil
		}

		x, y, err := dockNextMoveCompute(ctx, cam)
		if err != nil {
			logger.Infof("failed to compute, will try again: %s", err)
			continue
		}
		logger.Debugf("x: %v y: %v\n", x, y)

		angle := x * -15
		logger.Debugf("turning %v degrees", angle)
		err = base.Spin(ctx, angle, 10, true)
		if err != nil {
			return err
		}

		amount := 100.0 - (100.0 * y)
		logger.Debugf("moved %v millis", amount)
		err = base.MoveStraight(ctx, int(-1*amount), 50, true)
		if err != nil {
			return err
		}

		tries = 0
	}

	return fmt.Errorf("failed to dock")
}

// return delta x, delta y, error
func dockNextMoveCompute(ctx context.Context, cam gostream.ImageSource) (float64, float64, error) {
	ctx, span := trace.StartSpan(ctx, "dockNextMoveCompute")
	defer span.End()

	logger.Debugf("dock - next")
	img, closer, err := cam.Next(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer closer()

	logger.Debugf("dock - convert")
	ri := rimage.ConvertImage(img)

	logger.Debugf("dock - findBlack")
	p, _, err := findBlack(ctx, ri, nil)
	if err != nil {
		return 0, 0, err
	}

	logger.Debugf("black: %v", p)

	x := 2 * float64((ri.Width()/2)-p.X) / float64(ri.Width())
	y := 2 * float64((ri.Height()/2)-p.Y) / float64(ri.Height())
	return x, y, nil
}

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

			x, err := segmentation.ShapeWalk(img,
				image.Point{x, y},
				segmentation.ShapeWalkOptions{
					SkipCleaning: true,
					//Debug: true,
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

	return image.Point{}, nil, fmt.Errorf("no black found")
}

// ------

type Rover struct {
	theBoard  board.Board
	pan, tilt board.Servo
}

func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(uint8(PanCenter+(left*-30)), uint8(TiltCenter-20))
}

func (r *Rover) neckPosition(pan, tilt uint8) error {
	logger.Debugf("neckPosition to %v %v", pan, tilt)
	return multierr.Combine(r.pan.Move(context.TODO(), pan), r.tilt.Move(context.TODO(), tilt))
}

func (r *Rover) Ready(theRobot api.Robot) error {
	logger.Debugf("minirover2 Ready called")
	cam := theRobot.CameraByName("front")
	if cam == nil {
		return fmt.Errorf("no camera named front")
	}

	// doing this in a goroutine so i can see camera and servo data in web ui, but probably not right long term
	if false {
		go func() {
			for {
				time.Sleep(time.Second)
				var depthErr bool
				func() {
					img, release, err := cam.Next(context.Background())
					if err != nil {
						logger.Debugf("error from camera %s", err)
						return
					}
					defer release()
					pc := rimage.ConvertToImageWithDepth(img)
					if pc.Depth == nil {
						logger.Warnf("no depth data")
						depthErr = true
						return
					}
					err = pc.WriteTo(fmt.Sprintf("data/rover-centering-%d.both.gz", time.Now().Unix()))
					if err != nil {
						logger.Debugf("error writing %s", err)
					}
				}()
				if depthErr {
					return
				}
			}
		}()
	}

	return nil
}

func NewRover(r api.Robot, theBoard board.Board) (*Rover, error) {
	rover := &Rover{theBoard: theBoard}
	rover.pan = theBoard.Servo("pan")
	rover.tilt = theBoard.Servo("tilt")

	if false {
		go func() {
			for {
				time.Sleep(1500 * time.Millisecond)
				err := rover.neckCenter()
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(-1)
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(1)
				if err != nil {
					panic(err)
				}

			}
		}()
	} else {
		err := rover.neckCenter()
		if err != nil {
			return nil, err
		}
	}

	theLidar := r.LidarDeviceByName("lidarBase")
	if false && theLidar != nil {
		go func() {
			for {
				time.Sleep(time.Second)

				ms, err := theLidar.Scan(context.Background(), lidar.ScanOptions{})
				if err != nil {
					logger.Infof("theLidar scan failed: %s", err)
					continue
				}

				logger.Debugf("fowrad distance %#v", ms[0])
			}
		}()
	}

	logger.Debug("rover ready")

	return rover, nil
}

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	flag.Parse()

	exp := egoutil.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	cfg, err := api.ReadConfig("samples/minirover/config.json")
	if err != nil {
		return err
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	rover, err := NewRover(myRobot, myRobot.BoardByName("local"))
	if err != nil {
		return err
	}
	err = rover.Ready(myRobot)
	if err != nil {
		return err
	}

	options := web.NewOptions()
	options.AutoTile = false
	options.Pprof = true
	return web.RunWeb(context.Background(), myRobot, options, logger)
}
