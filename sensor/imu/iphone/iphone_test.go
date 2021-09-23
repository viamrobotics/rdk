package iphone_test

import (
	"context"
	"encoding/json"
	"github.com/edaniels/golog"
	"go.viam.com/core/sensor/imu/iphone"
	"go.viam.com/test"
	"net"
	"testing"
	"time"
)

// Example data for fake iPhone server to repeatedly send.
var (
	rotationRateX = 1.01
	rotationRateY = 2.02
	rotationRateZ = 3.03
	pitch         = 4.04
	roll          = 5.05
	yaw           = 6.06
	heading       = 7.07
	goodIMUData   = iphone.IPhoneMeasurement{
		RotationRateX: &rotationRateX,
		RotationRateY: &rotationRateY,
		RotationRateZ: &rotationRateZ,
		Pitch:         &pitch,
		Roll:          &roll,
		Yaw:           &yaw,
		Heading:       &heading,
	}
)

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// Fail if host does not exist or is not reachable.
	_, err := iphone.New("fake_host:(", logger)
	test.That(t, err, test.ShouldNotBeNil)

	// Succeed if host does exist.
	// Create dummy host.
	l, err := getIphoneServer()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, err = iphone.New(":3000", logger)
	test.That(t, err, test.ShouldBeNil)
}

func TestAngularVelocities(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// Fail if IPhone is serving bad AngularVelocity data.
	l, err := getIphoneServer()
	if err != nil {
		t.Fatal(err)
	}
	ip, err := iphone.New(":3000", logger)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ip.AngularVelocities(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	// Succeed if IPhone is serving valid AngularVelocity Data, and confirm that the
	// data is what is expected.
	err = sendIMUData(l)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 100)
	ret, err := ip.AngularVelocities(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret[0], test.ShouldEqual, rotationRateX)
	test.That(t, ret[1], test.ShouldEqual, rotationRateY)
	test.That(t, ret[2], test.ShouldEqual, rotationRateZ)
	l.Close()
}

// Creates IPhone server that serves invalid IMU Data.
func getIphoneServer() (net.Listener, error) {
	l, err := net.Listen("tcp", ":3000")
	if err != nil {
		return nil, err
	}
	return l, nil
}

func sendIMUData(l net.Listener) error {
	conn, err := l.Accept()
	if err != nil {
		return err
	}
	for {
		b, err := json.Marshal(goodIMUData)
		if err != nil {
			return err
		}
		_, err = conn.Write(b)
		if err != nil {
			return err
		}
		conn.Write([]byte("\n"))
		time.Sleep(time.Millisecond * 10)
	}
}
