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

	failingEncoder.GetPositionFunc = func(ctx context.Context, positionType *pb.PositionType, extra map[string]interface{}) (float64, error) {
		return 0, errors.New("position unavailable")
	}
	req = pb.GetPositionRequest{Name: failEncoderName}
	resp, err = encoderServer.GetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingEncoder.GetPositionFunc = func(ctx context.Context, positionType *pb.PositionType, extra map[string]interface{}) (float64, error) {
		return 42.0, nil
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
	req = pb.ResetPositionRequest{Name: failEncoderName, Offset: 1.1}
	resp, err = encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	workingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}
	req = pb.ResetPositionRequest{Name: testEncoderName, Offset: 1.1}
	resp, err = encoderServer.ResetPosition(context.Background(), &req)
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

	req := pb.ResetPositionRequest{Name: testEncoderName, Offset: 1.1, Extra: ext}
	resp, err := encoderServer.ResetPosition(context.Background(), &req)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualExtra["foo"], test.ShouldEqual, expectedExtra["foo"])
	test.That(t, actualExtra["baz"], test.ShouldResemble, expectedExtra["baz"])
}
