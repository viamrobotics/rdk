package gripper

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Gripper struct {
	conn net.Conn
}

func NewGripper(host string) (*Gripper, error) {
	conn, err := net.Dial("tcp", host+":63352")
	if err != nil {
		return nil, err
	}
	g := &Gripper{conn}

	g.Set("ACT", "1")   // robot activate
	g.Set("GTO", "1")   // gripper activate
	g.Set("FOR", "200") // force (0-255)
	g.Set("SPE", "200") // speed (0-255)

	// wait for init
	time.Sleep(1500 * time.Millisecond) // TODO: how to make better

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

func (g *Gripper) Set(what string, to string) (string, error) {
	return g.Send(fmt.Sprintf("SET %s %s\r\n", what, to))
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

func (g *Gripper) SetPos(pos string) (bool, error) {
	_, err := g.Set("POS", pos)
	if err != nil {
		return false, err
	}

	prev := ""

	for {
		x, err := g.Get("POS")
		if err != nil {
			return false, err
		}
		fmt.Printf("got: [%s]\n", x)
		if x == "POS "+pos {
			return true, nil
		}

		if prev == x {
			return false, nil
		}
		prev = x

		time.Sleep(100 * time.Millisecond)
	}

}

func (g *Gripper) Open() (bool, error) {
	return g.SetPos("3")
}

func (g *Gripper) Close() (bool, error) {
	return g.SetPos("249")
}
