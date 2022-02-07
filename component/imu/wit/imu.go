// Package wit implements a wit IMU.
package wit

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	slib "github.com/jacobsa/go-serial/serial"
	"go.viam.com/utils"

	rdkerr "github.com/pkg/errors"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

const model = "wit"

func init() {
	registry.RegisterComponent(imu.Subtype, model, registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			//nolint:contextcheck
			return NewWit(r, config, logger)
		},
	})
}

type acceleration struct {
	X, Y, Z float64
}

type velocity struct {
	X, Y, Z float64
}

type position struct {
	X, Y, Z float64
}

type wit struct {
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	acceleration    acceleration
	lastError       error

	mu sync.Mutex

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  golog.Logger
}

func (i *wit) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	i.mu.Lock()
	rdkerr.Errorf("angVel:", i.angularVelocity, "orient:", i.orientation)

	defer i.mu.Unlock()
	return i.angularVelocity, i.lastError
}

func (i *wit) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return &i.orientation, i.lastError
}

func (i *wit) GetReadings(ctx context.Context) ([]interface{}, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return []interface{}{i.angularVelocity, i.orientation}, i.lastError
}

// NewWit creates a new Wit IMU.
func NewWit(r robot.Robot, config config.Component, logger golog.Logger) (imu.IMU, error) {
	options := slib.OpenOptions{
		BaudRate:        9600, // 115200, wanted to set higher but windows software was being weird about it
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	options.PortName = config.Attributes.String("port")
	if options.PortName == "" {
		return nil, errors.New("wit imu needs a port")
	}

	port, err := slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(port)

	var i wit

	var ctx context.Context
	ctx, i.cancelFunc = context.WithCancel(context.Background())
	i.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(port.Close)
		defer i.activeBackgroundWorkers.Done()

		for {
			if ctx.Err() != nil {
				return
			}

			line, err := portReader.ReadString('U')

			func() {
				i.mu.Lock()
				defer i.mu.Unlock()

				if err != nil {
					i.lastError = err
				} else {
					i.lastError = i.parseWIT(line)
				}
			}()
		}
	})
	return &i, nil
}

func scale(a, b byte, r float64) float64 {
	x := float64(int(b)<<8|int(a)) / 32768.0 // 0 -> 2
	x *= r                                   // 0 -> 2r
	x += r
	x = math.Mod(x, r*2)
	x -= r

	//fmt.Println(x)
	return x
}

func (i *wit) parseWIT(line string) error {
	//fmt.Println(line[0])
	if line[0] == 0x52 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu angularVelocity %d %v", len(line), line)
		}
		i.angularVelocity.X = scale(line[1], line[2], 2000)
		i.angularVelocity.Y = scale(line[3], line[4], 2000)
		i.angularVelocity.Z = scale(line[5], line[6], 2000)
		//fmt.Println(i.angularVelocity)
	}

	if line[0] == 0x53 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu orientation %d %v", len(line), line)
		}

		i.orientation.Roll = rutils.DegToRad(scale(line[1], line[2], 180))
		i.orientation.Pitch = rutils.DegToRad(scale(line[3], line[4], 180))
		i.orientation.Yaw = rutils.DegToRad(scale(line[5], line[6], 180))
		//fmt.Println(i.orientation)
	}

	if line[0] == 0x51 { // TODO: Check for linear Velocity
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu orientation %d %v", len(line), line)
		}
		i.acceleration.X = scale(line[1], line[2], 16)
		i.acceleration.Y = scale(line[3], line[4], 16)
		i.acceleration.Z = scale(line[5], line[6], 16)
		//fmt.Println(i.acceleration)
		//fmt.Println(i.posFromAccn())
		i.posFromAccn()
	}

	return nil
}

func (i *wit) posFromAccn() float64 {
	prevAccn := i.acceleration.X
	// fmt.Println("prevAcc init", prevAccn)
	start := time.Now()
	currAcc := i.acceleration.X
	// fmt.Println("currAccn init:", currAcc)
	elapsed := float64(start.Sub(start))
	fmt.Println("elapsed integral 1:", elapsed)
	vel := (currAcc - prevAccn) / 2 * elapsed
	// fmt.Println("vel init:", vel)
	elapsed = float64(start.Sub(start))
	fmt.Println("elapsed integral 2:", elapsed)
	pos := vel*elapsed + (prevAccn-currAcc)/2*elapsed*elapsed
	// fmt.Println("pos end":, pos)
	prevAccn = currAcc
	// fmt.Println("currAccn end":, currAccn)

	return pos
}

func (i *wit) Close() {
	i.cancelFunc()
	i.activeBackgroundWorkers.Wait()
}
