package motor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestPIDConfig(t *testing.T) {
	ctx := context.Background()

	for i, tc := range []struct {
		conf PIDConfig
		err  string
	}{
		{
			PIDConfig{
				Name:       "Test",
				Attributes: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
				Type:       "other",
			},
			"unsupported PID type other",
		},
		{
			PIDConfig{
				Name:       "Test",
				Attributes: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
				Type:       "basic",
			},
			"",
		},
	} {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			pid, err := CreatePID(&tc.conf)
			if pid != nil {
				c, err := pid.Config(ctx)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, tc.conf, test.ShouldResemble, *c)
			} else {
				test.That(t, err.Error(), test.ShouldEqual, tc.err)
			}
		})
	}
}

func TestPIDBasicIntegralWindup(t *testing.T) {
	ctx := context.Background()
	pid, err := CreatePID(&PIDConfig{
		Name:       "Test",
		Attributes: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
		Type:       "basic",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid, test.ShouldNotBeNil)
	for i := 0; i < 50; i++ {
		dt := time.Duration(1000000 * 10)
		out, ok := pid.Output(ctx, dt, 1000, 0)
		if i < 47 {
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, out, test.ShouldEqual, 100.0)
		} else {
			test.That(t, pid.(*BasicPID).sat, test.ShouldEqual, 1)
			test.That(t, pid.(*BasicPID).int, test.ShouldBeGreaterThanOrEqualTo, 100)
			out, ok := pid.Output(ctx, dt, 1000, 1000)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.(*BasicPID).sat, test.ShouldEqual, 1)
			test.That(t, pid.(*BasicPID).int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out, test.ShouldEqual, 0.0)
			out, ok = pid.Output(ctx, dt, 1000, 1000)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.(*BasicPID).sat, test.ShouldEqual, 0)
			test.That(t, pid.(*BasicPID).int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out, test.ShouldEqual, 100.0)
			break
		}
	}
	err = pid.Reset()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.(*BasicPID).sat, test.ShouldEqual, 0)
	test.That(t, pid.(*BasicPID).int, test.ShouldEqual, 0)
	test.That(t, pid.(*BasicPID).error, test.ShouldEqual, 0)
}

func TestUpdateConfig(t *testing.T) {
	ctx := context.Background()
	pid, err := CreatePID(&PIDConfig{
		Name:       "Test",
		Attributes: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
		Type:       "basic",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid, test.ShouldNotBeNil)
	c, err := pid.Config(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c, test.ShouldResemble, &PIDConfig{
		Name:       "Test",
		Attributes: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
		Type:       "basic",
	})
	err = pid.UpdateConfig(ctx, PIDConfig{
		Attributes: config.AttributeMap{"Kd": 0.1234567, "Kp": 10.122, "Ki": 22.33445},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c, test.ShouldResemble, &PIDConfig{
		Name:       "Test",
		Attributes: config.AttributeMap{"Kd": 0.1234567, "Kp": 10.122, "Ki": 22.33445},
		Type:       "basic",
	})
}
