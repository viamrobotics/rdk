package board

import (
	"context"
	"sync"
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

	intVal, err := i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))
	test.That(t, i.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	test.That(t, i.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))

	c := make(chan bool)
	i.AddCallback(c)

	go func() { i.Tick(context.Background(), true, nowNanosTest()) }()
	v := <-c
	test.That(t, v, test.ShouldBeTrue)

	go func() { i.Tick(context.Background(), true, nowNanosTest()) }()
	v = <-c
	test.That(t, v, test.ShouldBeTrue)
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
	test.That(t, i.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1))

	c1 := make(chan bool)
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
		case ret = <-c1:
		}
	}()
	test.That(t, i.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
	intVal, err = i.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(2))
	wg.Wait()
	c2 := make(chan bool)
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
		case ret2 = <-c2:
		}
	}()
	wg.Add(1)
	go func() {
		err := i.Tick(context.Background(), true, nowNanosTest())
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

	now := uint64(0)
	for i := 0; i < 20; i++ {
		test.That(t, s.Tick(context.Background(), true, now), test.ShouldBeNil)
		now += 1500 * 1000 // this is what we measure
		test.That(t, s.Tick(context.Background(), false, now), test.ShouldBeNil)
		now += 1000 * 1000 * 1000 // this is between measurements
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

	now := uint64(0)
	for i := 0; i < 20; i++ {
		test.That(t, s.Tick(context.Background(), true, now), test.ShouldBeNil)
		now += 1500 * 1000 // this is what we measure
		test.That(t, s.Tick(context.Background(), false, now), test.ShouldBeNil)
		now += 1000 * 1000 * 1000 // this is between measurements
	}

	intVal, err := s.Value(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, intVal, test.ShouldEqual, int64(1501))
}
