package gripper

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Gripper struct {
	conn net.Conn

	openLimit  string
	closeLimit string
}

func NewGripper(host string) (*Gripper, error) {
	conn, err := net.Dial("tcp", host+":63352")
	if err != nil {
		return nil, err
	}
	g := &Gripper{conn, "0", "255"}

	init := [][]string{
		[]string{"ACT", "1"},   // robot activate
		[]string{"GTO", "1"},   // gripper activate
		[]string{"FOR", "200"}, // force (0-255)
		[]string{"SPE", "255"}, // speed (0-255)
	}
	for _, i := range init {
		err = g.Set(i[0], i[1])
		if err != nil {
			return nil, err
		}

		// TODO: the next 5 lines are infuriatng, help!
		if i[0] == "ACT" {
			time.Sleep(1600 * time.Millisecond)
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	err = g.Calibrate() // TODO: should this live elsewhere?
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Gripper) Send(msg string) (string, error) {
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

func (g *Gripper) Set(what string, to string) error {
	res, err := g.Send(fmt.Sprintf("SET %s %s\r\n", what, to))
	if err != nil {
		return err
	}
	if res != "ack" {
		return fmt.Errorf("didn't get ack back, got [%s]", res)
	}
	return nil
}

func (g *Gripper) Get(what string) (string, error) {
	return g.Send(fmt.Sprintf("GET %s\r\n", what))
}

func (g *Gripper) read() (string, error) {
	buf := make([]byte, 128)
	x, err := g.conn.Read(buf)
	if err != nil {
		return "", err
	}
	if x > 100 {
		return "", fmt.Errorf("Gripper::read too much: %d\n", x)
	}
	if x == 0 {
		return "", nil
	}
	return strings.TrimSpace(string(buf[0:x])), nil
}

// --------------

// return true iff reached desired position
func (g *Gripper) SetPos(pos string) (bool, error) {
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
			prevCount = prevCount + 1
		} else {
			prevCount = 0
		}
		prev = x

		time.Sleep(100 * time.Millisecond)
	}

}

func (g *Gripper) Open() error {
	_, err := g.SetPos(g.openLimit)
	return err
}

func (g *Gripper) Close() error {
	_, err := g.SetPos(g.closeLimit)
	return err
}

// return true iff grabbed something
func (g *Gripper) Grab() (bool, error) {
	res, err := g.SetPos(g.closeLimit)
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

func (g *Gripper) Calibrate() error {
	err := g.Open()
	if err != nil {
		return err
	}

	x, err := g.Get("POS")
	if err != nil {
		return err
	}
	g.openLimit = x[4:]

	err = g.Close()
	if err != nil {
		return err
	}

	x, err = g.Get("POS")
	if err != nil {
		return err
	}
	g.closeLimit = x[4:]

	fmt.Printf("limits %s %s\n", g.openLimit, g.closeLimit)
	return nil
}
