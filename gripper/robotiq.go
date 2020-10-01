package gripper

import (
	"net"
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
	return &Gripper{conn}, nil
}

func (g *Gripper) Set(what string, to string) error {
	_, err := g.conn.Write([]byte("SET " + what + " " + to + "\r\n"))
	// TODO: rather than sleeping, should ask if we've made it there yet
	time.Sleep(10 * time.Millisecond)
	return err
}
