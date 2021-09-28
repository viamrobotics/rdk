// Package main is the work-in-progress robotic boat from Viam.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/go-errors/errors"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/serial"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"

	geo "github.com/kellydunn/golang-geo"

	//"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var logger = golog.NewDevelopmentLogger("boat2")

type remoteControl interface {
	Signal(ctx context.Context, name string) (int64, error)
	Signals(ctx context.Context, name []string) (map[string]int64, error)
}

type rcRemoteControl struct {
	theBoard board.Board
}

func (rc *rcRemoteControl) Signal(ctx context.Context, name string) (int64, error) {
	r, ok := rc.theBoard.DigitalInterruptByName(name)
	if !ok {
		return 0, fmt.Errorf("no signal named %s", name)
	}
	return r.Value(ctx)
}

func (rc *rcRemoteControl) Signals(ctx context.Context, names []string) (map[string]int64, error) {
	m := map[string]int64{}

	for _, n := range names {
		val, err := rc.Signal(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("cannot read value of %s %w", n, err)
		}
		m[n] = val
	}

	return m, nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

type boat struct {
	myRobot robot.Robot
	rc      remoteControl

	squirt, steering, thrust motor.Motor
	middle                   float64
	steeringRange            float64

	myCompass compass.Compass

	mongoClient *mongo.Client
	waypoints   *mongo.Collection
}

func (b *boat) Off(ctx context.Context) error {
	return multierr.Combine(
		b.thrust.Off(ctx),
		b.squirt.Off(ctx),
	)
}

func (b *boat) GetBearing(ctx context.Context) (float64, error) {
	if b.myCompass != nil {
		dir, err := b.myCompass.Heading(ctx)
		return fixAngle(dir), err
	}

	if len(path) < 2 {
		return 0, errors.New("no gps data")
	}
	x := len(path)
	return fixAngle(path[x-2].BearingTo(path[x-1])), nil
}

// dir -1 -> 1
func (b *boat) Steer(ctx context.Context, dir float64) error {
	dir = b.steeringRange * dir
	dir *= .7 // was too aggressive
	dir += b.middle
	return b.steering.GoTo(ctx, 50, dir)
}

func newBoat(ctx context.Context, myRobot robot.Robot) (*boat, error) {
	var err error
	b := &boat{myRobot: myRobot}

	b.mongoClient, err = mongo.NewClient(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		return nil, err
	}
	err = b.mongoClient.Connect(ctx)
	if err != nil {
		return nil, err
	}

	b.waypoints = b.mongoClient.Database("boat").Collection("waypoints")

	bb, ok := myRobot.BoardByName("local")
	if !ok {
		return nil, errors.New("no local board")
	}
	b.rc = &rcRemoteControl{bb}

	// get all motors

	b.squirt, ok = myRobot.MotorByName("squirt")
	if !ok {
		return nil, errors.New("no squirt motor")
	}

	b.steering, ok = myRobot.MotorByName("steering")
	if !ok {
		return nil, errors.New("no steering motor")
	}

	b.thrust, ok = myRobot.MotorByName("thrust")
	if !ok {
		return nil, errors.New("no thrust motor")
	}

	err = b.Off(ctx)
	if err != nil {
		return nil, err
	}

	// calibrate steering
	err = b.steering.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 50, nil)
	if err != nil {
		return nil, err
	}

	bwdLimit, err := b.steering.Position(ctx)
	if err != nil {
		return nil, err
	}

	err = b.steering.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50, nil)
	if err != nil {
		return nil, err
	}

	fwdLimit, err := b.steering.Position(ctx)
	if err != nil {
		return nil, err
	}

	logger.Debugf("bwdLimit: %v fwdLimit: %v", bwdLimit, fwdLimit)

	b.steeringRange = (fwdLimit - bwdLimit) / 2
	b.middle = bwdLimit + b.steeringRange

	if b.steeringRange < 1 {
		return nil, fmt.Errorf("steeringRange only %v", b.steeringRange)
	}

	return b, multierr.Combine(b.thrust.Off(ctx), b.steering.GoTo(ctx, 50, b.middle))
}

func runRC(ctx context.Context, myBoat *boat) {
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return
		}

		vals, err := myBoat.rc.Signals(ctx, []string{"throttle", "direction", "speed", "mode"})
		if err != nil {
			logger.Errorw("error getting rc signal %w", err)
			continue
		}
		//logger.Debugf("vals: %v", vals)

		if vals["mode"] <= 1 {
			continue
		}

		squirtPower := float32(vals["throttle"]) / 100.0
		err = myBoat.squirt.Power(ctx, squirtPower)
		if err != nil {
			logger.Errorw("error turning on squirt: %w", err)
			continue
		}

		err = myBoat.Steer(ctx, float64(vals["direction"])/100.0)
		if err != nil {
			logger.Errorw("error turning: %w", err)
			continue
		}

		speed := float32(vals["speed"]) / 100.0
		speedDir := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		if speed < 0 {
			speed *= -1
			speedDir = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		}
		//fmt.Printf("speedDir: %v speed: %v\n", speedDir, speed)
		err = myBoat.thrust.Go(ctx, speedDir, speed)
		if err != nil {
			logger.Errorw("error thrusting: %w", err)
			continue
		}

	}
}

//var poleAcross = geo.NewPoint(40.7453889, -74.011)
//var outOfFinger = geo.NewPoint(40.7449719, -74.0109809)

var waypoints = []waypoint{
	{Lat: 40.745297, Long: -74.010916},
	{Lat: 40.7448830, Long: -74.0109611},
	{Lat: 40.7448805, Long: -74.0103623},
	{Lat: 40.7452645, Long: -74.0102802}, // middle
	{Lat: 40.7448805, Long: -74.0103623},
	{Lat: 40.7448830, Long: -74.0109611},
	{Lat: 40.745297, Long: -74.010916},
}

var path = []*geo.Point{}

func (b *boat) waypointReached(ctx context.Context) error {
	waypoints = waypoints[1:]
	return nil
	/*
		wp, err := b.nextWaypoint(ctx)
		if err != nil {
			return fmt.Errorf("can't mark waypoint reached: %w", err)
		}

		//_, err = b.waypoints.DeleteOne(ctx, bson.M{"_id" : wp.ID}, bson.M{ "$set" : bson.M{ "visited" : true } })
		_, err = b.waypoints.DeleteOne(ctx, bson.M{"_id" : wp.ID})
		return err
	*/
}

type waypoint struct {
	ID      int
	Visited bool
	Order   int
	Lat     float64
	Long    float64
}

func (wp *waypoint) ToPoint() *geo.Point {
	return geo.NewPoint(wp.Lat, wp.Long)
}

func (b *boat) nextWaypoint(ctx context.Context) (waypoint, error) {
	if len(waypoints) == 0 {
		return waypoint{}, errors.New("no more waypoint")
	}
	return waypoints[0], nil
	/*
		filter := bson.D{{ "visited" ,false }}
		cursor, err :=
			b.waypoints.Find(ctx, filter, options.Find().SetSort( bson.M{ "order" : -1 } ).SetLimit(1))
		if err != nil {
			return waypoint{}, fmt.Errorf("can't get next waypoint: %w", err)
		}

		all := []Waypoint{}
		err = cursor.All(ctx, &all)
		if err != nil {
			return waypoint{}, fmt.Errorf("can't get next waypoint: %w", err)
		}

		fmt.Printf("hi %v\n", all)

		if len(all) == 0 {
			return Waypoint{}, errors.New("no more waypoint")
		}

		return all[0], nil
	*/
}

func (b *boat) DirectionAndDistanceToGo(ctx context.Context) (float64, float64, error) {
	if len(path) == 0 {
		return 0, 0, nil
	}

	last := path[len(path)-1]

	wp, err := b.nextWaypoint(ctx)
	if err != nil {
		return 0, 0, err
	}

	goal := wp.ToPoint()

	return fixAngle(last.BearingTo(goal)), last.GreatCircleDistance(goal), nil
}

func fixAngle(a float64) float64 {
	for a < 0 {
		a += 360
	}
	for a > 360 {
		a -= 360
	}
	return a
}

func computeBearing(a, b float64) float64 {
	a = fixAngle(a)
	b = fixAngle(b)

	t := b - a
	if t < -180 {
		t += 360
	}

	if t > 180 {
		t -= 360
	}

	return t
}

func autoDrive(ctx context.Context, myBoat *boat) {
	for {
		if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
			return
		}

		vals, err := myBoat.rc.Signals(ctx, []string{"mode"})
		if err != nil {
			logger.Errorw("error getting rc signal %w", err)
			continue
		}

		if vals["mode"] >= 1 {
			continue
		}

		err = autoDriveOne(ctx, myBoat)
		if err != nil {
			logger.Infof("error driving: %s", err)
		}
	}
}

// returns if at target
func autoDriveOne(ctx context.Context, myBoat *boat) error {
	if len(path) <= 1 {
		return errors.New("no gps data")
	}

	currentHeading, err := myBoat.GetBearing(ctx)
	if err != nil {
		return err
	}

	bearingToGoal, distanceToGoal, err := myBoat.DirectionAndDistanceToGo(ctx)
	if err != nil {
		return err
	}

	if distanceToGoal < .005 {
		logger.Debug("i made it")
		return myBoat.waypointReached(ctx)
	}

	bearingDelta := computeBearing(bearingToGoal, currentHeading)
	steeringDir := -bearingDelta / 180.0

	logger.Debugf("currentHeading: %0.0f bearingToGoal: %0.0f distanceToGoal: %0.3f bearingDelta: %0.1f steeringDir: %0.2f",
		currentHeading, bearingToGoal, distanceToGoal, bearingDelta, steeringDir)

	err = myBoat.Steer(ctx, steeringDir)
	if err != nil {
		return fmt.Errorf("error turning: %w", err)
	}

	err = myBoat.thrust.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 0.7)
	if err != nil {
		return fmt.Errorf("erorr thrusting %w", err)
	}

	return nil
}

func trackGPS(ctx context.Context, myBoat *boat) {
	dev, err := serial.Open("/dev/ttyAMA0")
	if err != nil {
		logger.Debugf("canot open gps serial %s", err)
		return
	}
	defer dev.Close()

	r := bufio.NewReader(dev)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := r.ReadString('\n')
		if err != nil {
			logger.Fatalf("can't read gps serial %s", err)
		}

		s, err := nmea.Parse(line)
		if err != nil {
			logger.Debugf("can't parse nmea %s : %s", line, err)
			continue
		}

		gll, ok := s.(nmea.GLL)
		if ok {
			now := ToPoint(gll)
			path = append(path, now)
		}
	}
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := newBoat(ctx, myRobot)
	if err != nil {
		return err
	}

	go runRC(ctx, b)
	go trackGPS(ctx, b)
	go autoDrive(ctx, b)

	if err := webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		cancel()
		return err
	}
	return nil
}
