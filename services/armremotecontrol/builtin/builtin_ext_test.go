package builtin_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"go.viam.com/rdk/components/arm"
	fakearm "go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/arm/xarm"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/input/webgamepad"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/armremotecontrol/builtin"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

func TestSafetyMonitoring(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gamepadName := input.Named("barf")
	gamepad, err := webgamepad.NewController(ctx, nil, config.Component{}, logger)
	test.That(t, err, test.ShouldBeNil)

	myArmName := arm.Named("warf")
	fakeArm, err := fakearm.NewArm(
		config.Component{
			Name:                arm.Subtype.String(),
			ConvertedAttributes: &fakearm.AttrConfig{ArmModel: string(xarm.ModelName6DOF.Name)},
		},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	injectArm := &inject.Arm{LocalArm: fakeArm}
	myBase, err := arm.WrapWithReconfigurable(injectArm, myArmName)
	test.That(t, err, test.ShouldBeNil)

	moveToPositionFirst := make(chan struct{})
	injectArm.MoveToPositionFunc = func(ctx context.Context, to spatialmath.Pose, ws *referenceframe.WorldState, extra map[string]interface{}) error {
		close(moveToPositionFirst)
		return nil
	}

	svc, err := builtin.NewBuiltIn(ctx, registry.Dependencies{
		gamepadName: gamepad,
		myArmName:   myBase,
	}, config.Service{
		ConvertedAttributes: &builtin.ServiceConfig{
			ArmName:               myArmName.Name,
			InputControllerName:   gamepadName.Name,
			JointStep:             10.0,
			MMStep:                0.1,
			DegreeStep:            5.0,
			ControllerSensitivity: 5.0,
			ControllerModes: []builtin.ControllerMode{
				{
					ModeName: "endpoints",
					ControlMapping: map[string]input.Control{
						"x":     input.AbsoluteX,
						"y":     input.AbsoluteY,
						"z":     input.AbsoluteHat0X,
						"roll":  input.AbsoluteHat0Y,
						"pitch": input.AbsoluteRY,
						"yaw":   input.AbsoluteRX,
					},
				},
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	type triggerer interface {
		TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error
	}
	test.That(t, gamepad.(triggerer).TriggerEvent(ctx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   1,
	}, nil), test.ShouldBeNil)

	<-moveToPositionFirst

	safetyFirst := make(chan struct{})
	moveToPositionSecond := make(chan struct{})
	injectArm.MoveToPositionFunc = func(ctx context.Context, to spatialmath.Pose, ws *referenceframe.WorldState, extra map[string]interface{}) error {
		<-safetyFirst
		close(moveToPositionSecond)
		return nil
	}

	var stored sync.Once
	var storedCount int32
	var storedID uuid.UUID
	var storedResourceName resource.Name
	sess1 := session.New("ownerID", nil, time.Minute, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
			close(safetyFirst)
		})
	})
	nextCtx := session.ToContext(context.Background(), sess1)

	test.That(t, gamepad.(triggerer).TriggerEvent(nextCtx, input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteHat0X,
		Value:   2,
	}, nil), test.ShouldBeNil)

	<-moveToPositionSecond

	test.That(t, storedID, test.ShouldEqual, sess1.ID())
	test.That(t, storedResourceName, test.ShouldResemble, myArmName)
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}
