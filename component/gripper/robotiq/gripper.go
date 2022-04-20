// Package robotiq implements the gripper from robotiq.
package robotiq

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname = "robotiq"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Host string `json:"host"`
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(ctx, config.ConvertedAttributes.(*AttrConfig).Host, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(input.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// robotiqGripper TODO.
type robotiqGripper struct {
	conn net.Conn

	openLimit  string
	closeLimit string
	logger     golog.Logger
}

// newGripper TODO.
func newGripper(ctx context.Context, host string, logger golog.Logger) (*robotiqGripper, error) {
	conn, err := net.Dial("tcp", host+":63352")
	if err != nil {
		return nil, err
	}
	g := &robotiqGripper{conn, "0", "255", logger}

	init := [][]string{
		{"ACT", "1"},   // robot activate
		{"GTO", "1"},   // gripper activate
		{"FOR", "200"}, // force (0-255)
		{"SPE", "255"}, // speed (0-255)
	}
	err = g.MultiSet(ctx, init)
	if err != nil {
		return nil, err
	}

	err = g.Calibrate(ctx) // TODO(erh): should this live elsewhere?
	if err != nil {
		return nil, err
	}

	return g, nil
}

// MultiSet TODO.
func (g *robotiqGripper) MultiSet(ctx context.Context, cmds [][]string) error {
	for _, i := range cmds {
		err := g.Set(i[0], i[1])
		if err != nil {
			return err
		}

		// TODO(erh): the next 5 lines are infuriatng, help!
		var waitTime time.Duration
		if i[0] == "ACT" {
			waitTime = 1600 * time.Millisecond
		} else {
			waitTime = 500 * time.Millisecond
		}
		if !utils.SelectContextOrWait(ctx, waitTime) {
			return ctx.Err()
		}
	}

	return nil
}

// Send TODO.
func (g *robotiqGripper) Send(msg string) (string, error) {
	_, err := g.conn.Write([]byte(msg))
	if err != nil {
		return "", err
	}

	res, err := g.read()
	if err != nil {
		return "", err
	}

	return res, err
}

// Set TODO.
func (g *robotiqGripper) Set(what string, to string) error {
	res, err := g.Send(fmt.Sprintf("SET %s %s\r\n", what, to))
	if err != nil {
		return err
	}
	if res != "ack" {
		return errors.Errorf("didn't get ack back, got [%s]", res)
	}
	return nil
}

// Get TODO.
func (g *robotiqGripper) Get(what string) (string, error) {
	return g.Send(fmt.Sprintf("GET %s\r\n", what))
}

func (g *robotiqGripper) read() (string, error) {
	buf := make([]byte, 128)
	x, err := g.conn.Read(buf)
	if err != nil {
		return "", err
	}
	if x > 100 {
		return "", errors.Errorf("read too much: %d", x)
	}
	if x == 0 {
		return "", nil
	}
	return strings.TrimSpace(string(buf[0:x])), nil
}

// SetPos returns true iff reached desired position.
func (g *robotiqGripper) SetPos(ctx context.Context, pos string) (bool, error) {
	err := g.Set("POS", pos)
	if err != nil {
		return false, err
	}

	prev := ""
	prevCount := 0

	for {
		x, err := g.Get("POS")
		if err != nil {
			return false, err
		}
		if x == "POS "+pos {
			return true, nil
		}

		if prev == x {
			if prevCount >= 5 {
				return false, nil
			}
			prevCount++
		} else {
			prevCount = 0
		}
		prev = x

		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
			return false, ctx.Err()
		}
	}
}

// Open TODO.
func (g *robotiqGripper) Open(ctx context.Context) error {
	_, err := g.SetPos(ctx, g.openLimit)
	return err
}

// Close TODO.
func (g *robotiqGripper) Close(ctx context.Context) error {
	_, err := g.SetPos(ctx, g.closeLimit)
	return err
}

// Grab returns true iff grabbed something.
func (g *robotiqGripper) Grab(ctx context.Context) (bool, error) {
	res, err := g.SetPos(ctx, g.closeLimit)
	if err != nil {
		return false, err
	}
	if res {
		// we closed, so didn't grab anything
		return false, nil
	}

	// we didn't close, let's see if we actually got something
	val, err := g.Get("OBJ")
	if err != nil {
		return false, err
	}
	return val == "OBJ 2", nil
}

// Calibrate TODO.
func (g *robotiqGripper) Calibrate(ctx context.Context) error {
	err := g.Open(ctx)
	if err != nil {
		return err
	}

	x, err := g.Get("POS")
	if err != nil {
		return err
	}
	g.openLimit = x[4:]

	err = g.Close(ctx)
	if err != nil {
		return err
	}

	x, err = g.Get("POS")
	if err != nil {
		return err
	}
	g.closeLimit = x[4:]

	g.logger.Debugf("limits %s %s", g.openLimit, g.closeLimit)
	return nil
}
