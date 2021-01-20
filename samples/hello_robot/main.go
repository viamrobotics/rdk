package main

import (
	"context"
	"flag"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/echolabsinc/robotcore/lidar/rplidar"
	"github.com/echolabsinc/robotcore/robot"
	"github.com/echolabsinc/robotcore/vision"

	"github.com/edaniels/golog"
	"github.com/sbinet/go-python"
)

func init() {
	err := python.Initialize()
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	flag.Parse()

	srcURL := "127.0.0.1"
	if flag.NArg() >= 1 {
		srcURL = flag.Arg(0)
	}

	lidarDevPath := "/dev/hello-lrf"
	if flag.NArg() >= 2 {
		lidarDevPath = flag.Arg(1)
	}

	helloRobot := NewHelloRobot()
	helloRobot.Startup()
	defer helloRobot.Stop()

	lidarDev, err := rplidar.NewRPLidar(lidarDevPath)
	if err != nil {
		panic(err)
	}
	lidarDev.Start()

	theRobot := robot.NewBlankRobot()
	theRobot.AddBase(helloRobot.Base(), robot.Component{})
	theRobot.AddCamera(vision.NewIntelServerSource(srcURL, 8181, nil), robot.Component{})
	theRobot.AddLidar(lidarDev, robot.Component{})

	defer theRobot.Close()

	mux := http.NewServeMux()

	webCloser, err := robot.InstallWeb(mux, theRobot)
	if err != nil {
		panic(err)
	}

	httpServer := &http.Server{
		Addr:           ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		webCloser()
		httpServer.Shutdown(context.Background())
	}()

	golog.Global.Debug("going to listen")
	golog.Global.Fatal(httpServer.ListenAndServe())
}

type HelloRobot struct {
	robotObj *python.PyObject
}

func NewHelloRobot() *HelloRobot {
	transportMod := python.PyImport_ImportModule("stretch_body.transport")
	transportMod.SetAttr(python.PyString_FromString("dbg_on"), python.PyInt_FromLong(0))
	robotMod := python.PyImport_ImportModule("stretch_body.robot")
	robot := robotMod.CallMethod("Robot")
	return &HelloRobot{robotObj: robot}
}

func (hr *HelloRobot) Startup() {
	hr.robotObj.CallMethod("startup")
}

func (hr *HelloRobot) Stop() {
	hr.robotObj.CallMethod("stop")
}

func (hr *HelloRobot) Home() {
	hr.robotObj.CallMethod("home")
}

func (hr *HelloRobot) pushCommand() {
	hr.robotObj.CallMethod("push_command")
}

type HelloRobotArm struct {
	robot  *HelloRobot
	armObj *python.PyObject
}

func (hr *HelloRobot) Arm() *HelloRobotArm {
	arm := hr.robotObj.GetAttrString("arm")
	return &HelloRobotArm{robot: hr, armObj: arm}
}

const armMoveSpeed = 1.0 / 4 // m/sec

func (hra *HelloRobotArm) MoveBy(meters float64) {
	hra.armObj.CallMethod("move_by", python.PyFloat_FromDouble(meters))
	hra.robot.pushCommand()
	time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/armMoveSpeed)) * time.Second)
}

type HelloRobotBase struct {
	robot   *HelloRobot
	baseObj *python.PyObject
}

func (hr *HelloRobot) Base() *HelloRobotBase {
	base := hr.robotObj.GetAttrString("base")
	return &HelloRobotBase{robot: hr, baseObj: base}
}

func (hrb *HelloRobotBase) MoveStraight(distanceMM int, speed int, block bool) error {
	if speed != 0 {
		golog.Global.Info("HelloRobotBase.MoveStraight does not support speed")
	}
	hrb.TranslateBy(float64(distanceMM)/1000, block)
	return nil
}

func (hrb *HelloRobotBase) Spin(degrees int, power int, block bool) error {
	if power != 0 {
		golog.Global.Info("HelloRobotBase.Spin does not support power")
	}
	hrb.RotateBy(-float64(degrees), block)
	return nil
}

func (hrb *HelloRobotBase) Stop() error {
	hrb.baseObj.CallMethod("stop")
	return nil
}

func (hrb *HelloRobotBase) Close() {
	hrb.Stop()
}

const baseTranslateSpeed = 1.0 / 4 // m/sec

func (hrb *HelloRobotBase) TranslateBy(meters float64, block bool) {
	hrb.baseObj.CallMethod("translate_by", python.PyFloat_FromDouble(meters))
	hrb.robot.pushCommand()
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/baseTranslateSpeed)) * time.Second)
	}
}

const baseRotateSpeed = 2 * math.Pi / 14 // rad/sec

// degrees ccw
func (hrb *HelloRobotBase) RotateBy(degrees float64, block bool) {
	rads := degrees * math.Pi / 180
	hrb.baseObj.CallMethod("rotate_by", python.PyFloat_FromDouble(rads))
	hrb.robot.pushCommand()
	if block {
		time.Sleep(time.Duration(math.Ceil(math.Abs(rads)/baseRotateSpeed)) * time.Second)
	}
}
