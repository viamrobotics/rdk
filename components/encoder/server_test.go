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
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.EncoderServiceServer, *inject.Encoder, *inject.Encoder, error) {
	injectEncoder1 := &inject.Encoder{}
	injectEncoder2 := &inject.Encoder{}

	resourceMap := map[resource.Name]interface{}{
		encoder.Named(testEncoderName): injectEncoder1,
		encoder.Named(failEncoderName): injectEncoder2,
		encoder.Named(fakeEncoderName): "not a encoder",
	}

	injectSvc, err := subtype.New(resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return encoder.NewServer(injectSvc), injectEncoder1, injectEncoder2, nil
}

func TestServerGetPosition(t *testing.T) {
	encoderServer, workingEncoder, failingEncoder, _ := newServer()

	// fails on a bad encoder
	req := pb.GetPositionRequest{Name: fakeEncoderName}
	resp, err := encoderServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	failingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType *encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 0, encoder.PositionTypeUNSPECIFIED, errors.New("position unavailable")
	}
	req = pb.GetPositionRequest{Name: failEncoderName}
	resp, err = encoderServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType *encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 42.0, encoder.PositionTypeUNSPECIFIED, nil
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
		return errors.New("set to zero failed")
	}
	req = pb.ResetPositionRequest{Name: failEncoderName}
	resp, err = encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

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

	failingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		return nil, errors.New("properties not found")
	}
	req = pb.GetPropertiesRequest{Name: failEncoderName}
	resp, err = encoderServer.GetProperties(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		return map[encoder.Feature]bool{
			encoder.TicksCountSupported:   true,
			encoder.AngleDegreesSupported: false,
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
