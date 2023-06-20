package encoder_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errPositionUnavailable = errors.New("position unavailable")
	errSetToZeroFailed     = errors.New("set to zero failed")
	errPropertiesNotFound  = errors.New("properties not found")
	errGetPropertiesFailed = errors.New("get properties failed")
)

func newServer() (pb.EncoderServiceServer, *inject.Encoder, *inject.Encoder, error) {
	injectEncoder1 := &inject.Encoder{}
	injectEncoder2 := &inject.Encoder{}

	resourceMap := map[resource.Name]encoder.Encoder{
		encoder.Named(testEncoderName): injectEncoder1,
		encoder.Named(failEncoderName): injectEncoder2,
	}

	injectSvc, err := resource.NewAPIResourceCollection(encoder.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return encoder.NewRPCServiceServer(injectSvc).(pb.EncoderServiceServer), injectEncoder1, injectEncoder2, nil
}

func TestServerGetPosition(t *testing.T) {
	encoderServer, workingEncoder, failingEncoder, _ := newServer()

	// fails on a bad encoder
	req := pb.GetPositionRequest{Name: fakeEncoderName}
	resp, err := encoderServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

	failingEncoder.PositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 0, encoder.PositionTypeUnspecified, errPositionUnavailable
	}

	// Position unavailable test
	req = pb.GetPositionRequest{Name: failEncoderName}
	resp, err = encoderServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, errPositionUnavailable)

	workingEncoder.PositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 42.0, encoder.PositionTypeUnspecified, nil
	}
	req = pb.GetPositionRequest{Name: testEncoderName}
	resp, err = encoderServer.GetPosition(context.Background(), &req)
	test.That(t, float64(resp.Value), test.ShouldEqual, 42.0)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerResetPosition(t *testing.T) {
	encoderServer, workingEncoder, failingEncoder, _ := newServer()

	// fails on a bad encoder
	req := pb.ResetPositionRequest{Name: fakeEncoderName}
	resp, err := encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errSetToZeroFailed
	}
	req = pb.ResetPositionRequest{Name: failEncoderName}
	resp, err = encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, errSetToZeroFailed)

	workingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}
	req = pb.ResetPositionRequest{Name: testEncoderName}
	resp, err = encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerGetProperties(t *testing.T) {
	encoderServer, workingEncoder, failingEncoder, _ := newServer()

	// fails on a bad encoder
	req := pb.GetPropertiesRequest{Name: fakeEncoderName}
	resp, err := encoderServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingEncoder.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		return encoder.Properties{}, errPropertiesNotFound
	}
	req = pb.GetPropertiesRequest{Name: failEncoderName}
	resp, err = encoderServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, errPropertiesNotFound)

	workingEncoder.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		return encoder.Properties{
			TicksCountSupported:   true,
			AngleDegreesSupported: false,
		}, nil
	}
	req = pb.GetPropertiesRequest{Name: testEncoderName}
	resp, err = encoderServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

func TestServerExtraParams(t *testing.T) {
	encoderServer, workingEncoder, _, _ := newServer()

	var actualExtra map[string]interface{}
	workingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}

	expectedExtra := map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}}

	ext, err := protoutils.StructToStructPb(expectedExtra)
	test.That(t, err, test.ShouldBeNil)

	req := pb.ResetPositionRequest{Name: testEncoderName, Extra: ext}
	resp, err := encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualExtra["foo"], test.ShouldEqual, expectedExtra["foo"])
	test.That(t, actualExtra["baz"], test.ShouldResemble, expectedExtra["baz"])
}
