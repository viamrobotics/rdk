package board

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
)

func nowMicrosecondsTest() uint32 {
	return uint32(time.Now().UnixMicro())
}

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "i1",
		Formula: "(+ 1 raw)",
	}

	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)

	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))
	test.That(t, i.Tick(context.Background(), true, nowMicrosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	test.That(t, i.Tick(context.Background(), false, nowMicrosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))

	c := make(chan Tick)
	i.AddCallback(c)

	timeMicroSec := nowMicrosecondsTest()
	go func() { i.Tick(context.Background(), true, timeMicroSec) }()
	time.Sleep(1 * time.Microsecond)
	v := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampMicroSec, test.ShouldEqual, timeMicroSec)

	timeMicroSec = nowMicrosecondsTest()
	go func() { i.Tick(context.Background(), true, timeMicroSec) }()
	v = <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampMicroSec, test.ShouldEqual, timeMicroSec)

	i.RemoveCallback(c)

	c = make(chan Tick, 2)
	i.AddCallback(c)
	go func() {
		i.Tick(context.Background(), true, uint32(math.MaxUint32)-1)
		i.Tick(context.Background(), true, uint32(1))
	}()
	v = <-c
	v1 := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v1.High, test.ShouldBeTrue)
	test.That(t, v1.TimestampMicroSec-v.TimestampMicroSec, test.ShouldEqual, uint32(3))
}

func TestRemoveCallbackDigitalInterrupt(t *testing.T) {
	config := DigitalInterruptConfig{
		Name: "d1",
	}
	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(0))
	test.That(t, i.Tick(context.Background(), true, nowMicrosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c1 := make(chan Tick)
	test.That(t, c1, test.ShouldNotBeNil)
	i.AddCallback(c1)
	var wg sync.WaitGroup
	wg.Add(1)
	ret := false

	go func() {
		defer wg.Done()
		select {
		case <-context.Background().Done():
			return
		default:
		}
		select {
		case <-context.Background().Done():
			return
		case tick := <-c1:
			ret = tick.High
		}
	}()
	test.That(t, i.Tick(context.Background(), true, nowMicrosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	wg.Wait()
	c2 := make(chan Tick)
	test.That(t, c2, test.ShouldNotBeNil)
	i.AddCallback(c2)
	test.That(t, ret, test.ShouldBeTrue)

	i.RemoveCallback(c1)
	i.RemoveCallback(c1)

	ret2 := false
	result := make(chan bool, 1)
	go func() {
		defer wg.Done()
		select {
		case <-context.Background().Done():
			return
		default:
		}
		select {
		case <-context.Background().Done():
			return
		case tick := <-c2:
			ret2 = tick.High
		}
	}()
	wg.Add(1)
	go func() {
		err := i.Tick(context.Background(), true, nowMicrosecondsTest())
		if err != nil {
			result <- true
		}
		result <- true
	}()
	select {
	case <-time.After(1 * time.Second):
		ret = false
	case ret = <-result:
	}
	wg.Wait()
	test.That(t, ret, test.ShouldBeTrue)
	test.That(t, ret2, test.ShouldBeTrue)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(3))
}

func TestServoInterrupt(t *testing.T) {
	config := DigitalInterruptConfig{
		Name: "s1",
		Type: "servo",
	}

	s, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)

	now := uint32(0)
	for i := 0; i < 20; i++ {
		test.That(t, s.Tick(context.Background(), true, now), test.ShouldBeNil)
		now += 1500 // this is what we measure
		test.That(t, s.Tick(context.Background(), false, now), test.ShouldBeNil)
		now += 1000 * 1000 // this is between measurements
	}

	intVal, err := s.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1500))
}

func TestServoInterruptWithPP(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "s1",
		Type:    "servo",
		Formula: "(+ 1 raw)",
	}

	s, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)

	now := uint32(0)
	for i := 0; i < 20; i++ {
		test.That(t, s.Tick(context.Background(), true, now), test.ShouldBeNil)
		now += 1500 // this is what we measure
		test.That(t, s.Tick(context.Background(), false, now), test.ShouldBeNil)
		now += 1000 * 1000 // this is between measurements
	}

	intVal, err := s.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1501))
}
