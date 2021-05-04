package board

import (
	"testing"
	"time"

	"go.viam.com/test"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "i1",
		Formula: "(+ 1 raw)",
	}

	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, i.Config().Name, test.ShouldEqual, "i1")

	test.That(t, i.Value(), test.ShouldEqual, int64(1))
	i.Tick(true, nowNanosTest())
	test.That(t, i.Value(), test.ShouldEqual, int64(2))
	i.Tick(false, nowNanosTest())
	test.That(t, i.Value(), test.ShouldEqual, int64(2))

	c := make(chan bool)
	i.AddCallback(c)

	go func() { i.Tick(true, nowNanosTest()) }()
	v := <-c
	test.That(t, v, test.ShouldBeTrue)

	go func() { i.Tick(true, nowNanosTest()) }()
	v = <-c
	test.That(t, v, test.ShouldBeTrue)

}

func TestServoInterrupt(t *testing.T) {
	config := DigitalInterruptConfig{
		Name: "s1",
		Type: "servo",
	}

	s, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.Config().Name, test.ShouldEqual, "s1")

	now := uint64(0)
	for i := 0; i < 20; i++ {
		s.Tick(true, now)
		now += 1500 * 1000 // this is what we measure
		s.Tick(false, now)
		now += 1000 * 1000 * 1000 // this is between measuremenats
	}

	test.That(t, s.Value(), test.ShouldEqual, int64(1500))
}

func TestServoInterruptWithPP(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "s1",
		Type:    "servo",
		Formula: "(+ 1 raw)",
	}

	s, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s.Config().Name, test.ShouldEqual, "s1")

	now := uint64(0)
	for i := 0; i < 20; i++ {
		s.Tick(true, now)
		now += 1500 * 1000 // this is what we measure
		s.Tick(false, now)
		now += 1000 * 1000 * 1000 // this is between measuremenats
	}

	test.That(t, s.Value(), test.ShouldEqual, int64(1501))
}
