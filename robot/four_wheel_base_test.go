package robot

import (
	"context"
	"testing"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

func TestFourWheelBase1(t *testing.T) {
	ctx := context.Background()
	r, err := NewRobot(ctx,
		api.Config{
			Boards: []board.Config{
				{
					Name:  "local",
					Model: "fake",
					Motors: []board.MotorConfig{
						{Name: "fr-m"},
						{Name: "fl-m"},
						{Name: "br-m"},
						{Name: "bl-m"},
					},
				},
			},
		},
		golog.Global,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreateFourWheelBase(context.Background(), r, api.Component{}, golog.Global)
	assert.NotNil(t, err)

	cfg := api.Component{
		Attributes: api.AttributeMap{
			"widthMillis":              100,
			"wheelCircumferenceMillis": 1000,
			"board":                    "local",
			"frontRight":               "fr-m",
			"frontLeft":                "fl-m",
			"backRight":                "br-m",
			"backLeft":                 "bl-m",
		},
	}
	basebase, err := CreateFourWheelBase(context.Background(), r, cfg, golog.Global)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, basebase)
	base, ok := basebase.(*fourWheelBase)
	assert.True(t, ok)

	t.Run("basics", func(t *testing.T) {
		temp, err := base.WidthMillis(ctx)
		assert.Nil(t, err)
		assert.Equal(t, 100, temp)
	})

	t.Run("math", func(t *testing.T) {
		d, rpm, rotations := base.straightDistanceToMotorInfo(1000, 1000)
		assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, d)
		assert.Equal(t, 60.0, rpm)
		assert.Equal(t, 1.0, rotations)

		d, rpm, rotations = base.straightDistanceToMotorInfo(-1000, 1000)
		assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, d)
		assert.Equal(t, 60.0, rpm)
		assert.Equal(t, 1.0, rotations)

		d, rpm, rotations = base.straightDistanceToMotorInfo(-1000, -1000)
		assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, d)
		assert.Equal(t, 60.0, rpm)
		assert.Equal(t, 1.0, rotations)
	})

	t.Run("waitForMotorsToStop", func(t *testing.T) {
		err := base.Stop(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = base.allMotors[0].Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1)
		if err != nil {
			t.Fatal(err)
		}
		isOn, err := base.allMotors[0].IsOn(context.Background())
		assert.Nil(t, err)
		assert.True(t, isOn)

		err = base.waitForMotorsToStop(ctx)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.False(t, isOn)
		}

		err = base.waitForMotorsToStop(ctx)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.False(t, isOn)
		}

	})

	assert.Nil(t, base.Close())

	t.Run("go no block", func(t *testing.T) {
		err := base.MoveStraight(ctx, 10000, 1000, false)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.True(t, isOn)
		}

		err = base.Stop(ctx)
		if err != nil {
			t.Fatal(err)
		}

	})

	t.Run("go block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		err := base.MoveStraight(ctx, 10000, 1000, true)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.False(t, isOn)
		}

	})

	t.Run("spin math", func(t *testing.T) {
		// i'm only testing pieces that are correct

		leftDirection, _, rotations := base.spinMath(90, 10)
		assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, leftDirection)
		assert.InEpsilon(t, .0785, rotations, .001)

		leftDirection, _, rotations = base.spinMath(-90, 10)
		assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, leftDirection)
		assert.InEpsilon(t, .0785, rotations, .001)

	})

	t.Run("spin no block", func(t *testing.T) {
		err := base.Spin(ctx, 5, 5, false)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.True(t, isOn)
		}

		err = base.Stop(ctx)
		if err != nil {
			t.Fatal(err)
		}

	})

	t.Run("spin block", func(t *testing.T) {
		go func() {
			time.Sleep(time.Millisecond * 10)
			err = base.Stop(ctx)
			if err != nil {
				panic(err)
			}
		}()

		err := base.Spin(ctx, 5, 5, true)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range base.allMotors {
			isOn, err := m.IsOn(context.Background())
			assert.Nil(t, err)
			assert.False(t, isOn)
		}

	})

}
