package builtin

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
)

type moveRequest struct {
	ComponentName string  `json:"component_name"`
	Speed         float64 `json:"speed"`
}

type fakeRes struct {
	resource.Named
	resource.TriviallyCloseable
	grabbed   bool
	lastMove  moveRequest
	lastExtra map[string]interface{}
}

func (f *fakeRes) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	f.grabbed = true
	f.lastExtra = extra
	return true, nil
}

func (f *fakeRes) Move(ctx context.Context, req moveRequest) error {
	f.lastMove = req
	return nil
}

func (f *fakeRes) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"pressed": true}, nil
}

func (f *fakeRes) SetPower(ctx context.Context, power float64, extra map[string]interface{}) error {
	if power > 1 {
		return errors.New("too much power")
	}
	return nil
}

func (f *fakeRes) Stop(ctx context.Context) {
}

func TestInvoke(t *testing.T) {
	res := &fakeRes{Named: generic.Named("g").AsNamed()}
	ctx := context.Background()

	out, err := invoke(ctx, res, "Grab", map[string]interface{}{"extra": map[string]interface{}{"k": "v"}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldResemble, map[string]interface{}{"result": true})
	test.That(t, res.grabbed, test.ShouldBeTrue)
	test.That(t, res.lastExtra, test.ShouldResemble, map[string]interface{}{"k": "v"})

	out, err = invoke(ctx, res, "Move", map[string]interface{}{"component_name": "gripper", "speed": 2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldResemble, map[string]interface{}{})
	test.That(t, res.lastMove, test.ShouldResemble, moveRequest{ComponentName: "gripper", Speed: 2})

	out, err = invoke(ctx, res, "Readings", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldResemble, map[string]interface{}{"pressed": true})

	_, err = invoke(ctx, res, "SetPower", map[string]interface{}{"power": 0.5})
	test.That(t, err, test.ShouldBeNil)
	_, err = invoke(ctx, res, "SetPower", []interface{}{2.0})
	test.That(t, err, test.ShouldBeError, errors.New("too much power"))
	_, err = invoke(ctx, res, "SetPower", map[string]interface{}{"a": 1, "b": 2})
	test.That(t, err, test.ShouldNotBeNil)

	out, err = invoke(ctx, res, "Stop", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldResemble, map[string]interface{}{})

	_, err = invoke(ctx, res, "Nope", nil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestFindResource(t *testing.T) {
	a := &fakeRes{Named: generic.Named("a").AsNamed()}
	b := &fakeRes{Named: resource.NewName(resource.APINamespaceRDK.WithComponentType("motor"), "a").AsNamed()}
	c := &fakeRes{Named: generic.Named("c").AsNamed()}
	deps := resource.Dependencies{a.Name(): a, b.Name(): b, c.Name(): c}

	got, err := findResource(deps, "c")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, c)

	_, err = findResource(deps, "a")
	test.That(t, err, test.ShouldNotBeNil)

	got, err = findResource(deps, "rdk:component:motor/a")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, b)

	_, err = findResource(deps, "zzz")
	test.That(t, err, test.ShouldNotBeNil)
}
