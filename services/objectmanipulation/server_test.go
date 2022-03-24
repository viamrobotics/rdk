package objectmanipulation_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/service/objectmanipulation/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(omMap map[resource.Name]interface{}) (pb.ObjectManipulationServiceServer, error) {
	omSvc, err := subtype.New(omMap)
	if err != nil {
		return nil, err
	}
	return objectmanipulation.NewServer(omSvc), nil
}

func TestServerDoGrab(t *testing.T) {
	grabRequest := &pb.DoGrabRequest{
		GripperName: "",
		Target:      referenceframe.PoseInFrameToProtobuf(referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())),
	}

	omMap := map[resource.Name]interface{}{}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DoGrab(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:object_manipulation\" not found"))

	// set up the robot with something that is not an objectmanipulation service
	omMap = map[resource.Name]interface{}{objectmanipulation.Name: "not object manipulation"}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.DoGrab(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("objectmanipulation.Service", "string"))

	// error
	injectOMS := &inject.ObjectManipulationService{}
	omMap = map[resource.Name]interface{}{
		objectmanipulation.Name: injectOMS,
	}
	server, err = newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	passedErr := errors.New("fake dograb error")
	injectOMS.DoGrabFunc = func(
		ctx context.Context,
		gripperName string,
		grabPose *referenceframe.PoseInFrame,
		obstacles []*referenceframe.GeometriesInFrame,
	) (bool, error) {
		return false, passedErr
	}

	_, err = server.DoGrab(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeError, passedErr)

	// returns response
	injectOMS.DoGrabFunc = func(
		ctx context.Context,
		gripperName string,
		grabPose *referenceframe.PoseInFrame,
		obstacles []*referenceframe.GeometriesInFrame,
	) (bool, error) {
		return true, nil
	}
	resp, err := server.DoGrab(context.Background(), grabRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetSuccess(), test.ShouldBeTrue)
}
