package limo

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/testutils/inject"
)

func TestLimoBaseConstructor(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return &fake.Motor{}, nil
	}

	c := make(chan []uint8, 100)

	cfg := &Config{
		TestChan: c,
	}

	_, err := CreateLimoBase(&Config{}, rlog.Logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg = &Config{
		DriveMode: "ackermann",
		TestChan:  c,
	}

	baseBase, err := CreateLimoBase(cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	base, ok := baseBase.(*limoBase)
	test.That(t, ok, test.ShouldBeTrue)
	width, _ := base.GetWidth(ctx)
	test.That(t, width, test.ShouldEqual, 172)
	base.Close(ctx)
}
