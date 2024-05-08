package pinwrappers

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
)

func nowNanosecondsTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := board.DigitalInterruptConfig{
		Name: "i1",
	}

	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)

	di := i.(*BasicDigitalInterrupt)

	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(0))
	test.That(t, Tick(context.Background(), i.(*BasicDigitalInterrupt), true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))
	test.That(t, Tick(context.Background(), i.(*BasicDigitalInterrupt), false, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c := make(chan board.Tick)
	AddCallback(di, c)

	timeNanoSec := nowNanosecondsTest()
	go func() { Tick(context.Background(), di, true, timeNanoSec) }()
	time.Sleep(1 * time.Microsecond)
	v := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampNanosec, test.ShouldEqual, timeNanoSec)

	timeNanoSec = nowNanosecondsTest()
	go func() { Tick(context.Background(), di, true, timeNanoSec) }()
	v = <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v.TimestampNanosec, test.ShouldEqual, timeNanoSec)

	RemoveCallback(di, c)

	c = make(chan board.Tick, 2)
	AddCallback(di, c)
	go func() {
		Tick(context.Background(), di, true, uint64(1))
		Tick(context.Background(), di, true, uint64(4))
	}()
	v = <-c
	v1 := <-c
	test.That(t, v.High, test.ShouldBeTrue)
	test.That(t, v1.High, test.ShouldBeTrue)
	test.That(t, v1.TimestampNanosec-v.TimestampNanosec, test.ShouldEqual, uint32(3))
}

func TestRemoveCallbackDigitalInterrupt(t *testing.T) {
	config := board.DigitalInterruptConfig{
		Name: "d1",
	}
	i, err := CreateDigitalInterrupt(config)
	test.That(t, err, test.ShouldBeNil)
	di := i.(*BasicDigitalInterrupt)
	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(0))
	test.That(t, Tick(context.Background(), di, true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c1 := make(chan board.Tick)
	test.That(t, c1, test.ShouldNotBeNil)
	AddCallback(di, c1)
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
	test.That(t, Tick(context.Background(), di, true, nowNanosecondsTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	wg.Wait()
	c2 := make(chan board.Tick)
	test.That(t, c2, test.ShouldNotBeNil)
	AddCallback(di, c2)
	test.That(t, ret, test.ShouldBeTrue)

	RemoveCallback(di, c1)

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
		err := Tick(context.Background(), di, true, nowNanosecondsTest())
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
