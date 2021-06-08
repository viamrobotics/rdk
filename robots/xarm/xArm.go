package xarm

import (
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
	
	"github.com/edaniels/golog"
)

var regMap = map[string]byte{
	"Version":     0x01,
	"Shutdown":    0x0A,
	"ToggleServo": 0x0B,
	"SetState":    0x0C,
	"GetState":    0x0D,
	"CmdCount":    0x0E,
	"GetError":    0x0F,
	"ClearError":  0x10,
	"ClearWarn":   0x11,
	"ToggleBrake": 0x12,
	"SetMode":     0x13,
	"MoveJoints":  0x17,
	"ZeroJoints":  0x19,
	"JointPos":    0x2A,
}

type cmd struct {
	tid    uint16
	prot   uint16
	reg    byte
	params []byte
}

type xArm6 struct {
	tid   uint16
	conn  net.Conn
	speed float32 //speed=20*π/180rad/s
	accel float32 //acceleration=500*π/180rad/s^2
	moveLock *sync.Mutex
}

//go:embed xArm6_kinematics.json
var xArm6modeljson []byte

func init() {
	registry.RegisterArm("xArm6", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		_, err := robot.AsMutable(r)
		if err != nil {
			return nil, err
		}
		return NewxArm6(ctx, config.Host, logger)
	})
}

func (c *cmd) bytes() []byte {
	var bin []byte
	uintBin := make([]byte, 2)
	binary.BigEndian.PutUint16(uintBin, c.tid)
	bin = append(bin, uintBin...)
	binary.BigEndian.PutUint16(uintBin, c.prot)
	bin = append(bin, uintBin...)
	binary.BigEndian.PutUint16(uintBin, 1 + uint16(len(c.params)))
	bin = append(bin, uintBin...)
	bin = append(bin, c.reg)
	bin = append(bin, c.params...)
	return bin
}

func float64ToByte(f float64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(f))
	return buf[:]
}

func float64ToByte32(f float64) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], math.Float32bits(float32(f)))
	return buf[:]
}

func float64fromByte32(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float64(float)
}


func NewxArm6(ctx context.Context, host string, logger golog.Logger) (arm.Arm, error) {
	conn, err := net.Dial("tcp", host + ":502")
	if err != nil {
		return &xArm6{}, err
	}
	mutex := &sync.Mutex{}
	xArm := xArm6{0, conn, 0.35, 8, mutex}

	err = xArm.start()

	return kinematics.NewArm(&xArm, xArm6modeljson, 4, logger)
}

func (x *xArm6) newCmd(reg byte) cmd {
	x.tid++
	return cmd{tid: x.tid, prot: 2, reg: reg}
}

func (x *xArm6) send(c cmd) (cmd, error) {
	b := c.bytes()
	x.moveLock.Lock()
	_, err := x.conn.Write(b)
	if err != nil{
		return cmd{}, err
	}
	r, err := x.response()
	x.moveLock.Unlock()
	return r, err
}

func (x *xArm6) response() (cmd, error) {
	// Read response header
	buf, err := utils.ReadBytes(context.Background(), x.conn, 7)
	if err != nil {
		return cmd{}, err
	}
	c := cmd{}
	c.tid = binary.BigEndian.Uint16(buf[0:2])
	c.prot = binary.BigEndian.Uint16(buf[2:4])
	c.reg = buf[6]
	length := binary.BigEndian.Uint16(buf[4:6])
	c.params, err = utils.ReadBytes(context.Background(), x.conn, int(length - 1))
	return c, err
}

func (x *xArm6) SetMotionState(state byte) error {
	c := x.newCmd(regMap["SetState"])
	c.params = append(c.params, state)
	_, err := x.send(c)
	return err
}

func (x *xArm6) SetMotionMode(mode byte) error {
	c := x.newCmd(regMap["SetMode"])
	c.params = append(c.params, mode)
	_, err := x.send(c)
	return err
}

func (x *xArm6) ToggleServos(enable bool) error {
	c := x.newCmd(regMap["ToggleServo"])
	var enByte byte
	if enable {
		enByte = 1
	}
	c.params = append(c.params, 8, enByte)
	_, err := x.send(c)
	return err
}

func (x *xArm6) ToggleBrake(enable bool) error {
	c := x.newCmd(regMap["ToggleBrake"])
	var enByte byte
	if enable {
		enByte = 1
	}
	c.params = append(c.params, 8, enByte)
	_, err := x.send(c)
	return err
}

func (x *xArm6) start() error {
	fmt.Println("servos on")
	err := x.ToggleServos(true)
	if err != nil {
		return err
	}
	x.SetMotionMode(1)
	x.SetMotionState(0)
	return err
}

func (x *xArm6) MotionWait() error {
	ready := false
	for !ready{
		time.Sleep(50 * time.Millisecond)
		c := x.newCmd(regMap["GetState"])
		sData, err := x.send(c)
		if err != nil{
			return err
		}
		if sData.params[1] != 1{
			ready = true
		}
	}
	return nil
}

func (x *xArm6) Close() error {
	fmt.Println("brakes on")
	err := x.ToggleBrake(false)
	if err != nil {
		return err
	}
	fmt.Println("servos off")
	err = x.ToggleServos(false)
	if err != nil {
		return err
	}
	x.SetMotionState(4)
	return err
}

func (x *xArm6) MoveToJointPositions(ctx context.Context, newPositions *pb.JointPositions) error {
	radians := arm.JointPositionsToRadians(newPositions)
	c := x.newCmd(regMap["MoveJoints"])
	jFloatBytes := make([]byte, 4)
	for _, jRad := range(radians){
		binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(float32(jRad)))
		c.params = append(c.params, jFloatBytes...)
	}
	// xarm 6 has 6 joints, but protocol needs 7
	c.params = append(c.params, 0,0,0,0)
	// Add speed, 0.35
	binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(float32(0.35)))
	c.params = append(c.params, jFloatBytes...)
	// Add accel, 8.7
	binary.LittleEndian.PutUint32(jFloatBytes, math.Float32bits(float32(8.7)))
	c.params = append(c.params, jFloatBytes...)
	
	// add motion time, 0
	c.params = append(c.params, 0,0,0,0)
	_, err := x.send(c)
	//~ time.Sleep(100 * time.Millisecond)
	x.MotionWait()
	return err
}

func (x *xArm6) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("not done yet")
}
func (x *xArm6) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return nil, errors.New("xArm6 low level doesn't support kinematics")
}

func (x *xArm6) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	return errors.New("xArm6 low level doesn't support kinematics")
}

func (x *xArm6) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	c := x.newCmd(regMap["JointPos"])
	
	jData, err := x.send(c)
	if err != nil{
		return &pb.JointPositions{}, err
	}
	var radians []float64
	for i := 0; i < 6; i++{
		idx := i*4 + 1
		radians = append(radians, float64fromByte32(jData.params[idx:idx + 4]))
	}
	
	return arm.JointPositionsFromRadians(radians), nil
}

