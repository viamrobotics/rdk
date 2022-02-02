package framesystem_test

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"gonum.org/v1/gonum/num/quat"
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
	viamgrpc "go.viam.com/rdk/grpc"
	servicepb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func ensurePartsAreEqual(part *config.FrameSystemPart, otherPart *config.FrameSystemPart) error {
	if part.Name != otherPart.Name {
		return errors.Errorf("part had name %s while other part had name %s", part.Name, otherPart.Name)
	}
	frameConfig := part.FrameConfig
	otherFrameConfig := otherPart.FrameConfig
	if frameConfig.Parent != otherFrameConfig.Parent {
		return errors.Errorf("part had parent %s while other part had parent %s", frameConfig.Parent, otherFrameConfig.Parent)
	}
	trans := frameConfig.Translation
	otherTrans := otherFrameConfig.Translation
	floatDisc := spatialmath.Epsilon
	transIsEqual := true
	transIsEqual = transIsEqual && utils.Float64AlmostEqual(trans.X, otherTrans.X, floatDisc)
	transIsEqual = transIsEqual && utils.Float64AlmostEqual(trans.Y, otherTrans.Y, floatDisc)
	transIsEqual = transIsEqual && utils.Float64AlmostEqual(trans.Z, otherTrans.Z, floatDisc)
	if !transIsEqual {
		return errors.New("translations of parts not equal")
	}
	orient := frameConfig.Orientation
	otherOrient := otherFrameConfig.Orientation

	switch {
	case orient == nil && otherOrient != nil:
		if !spatialmath.QuaternionAlmostEqual(otherOrient.Quaternion(), quat.Number{1, 0, 0, 0}, 1e-5) {
			return errors.New("orientations of parts not equal")
		}
	case otherOrient == nil:
		return errors.New("orientation not returned for other part")
	case !spatialmath.OrientationAlmostEqual(orient, otherOrient):
		return errors.New("orientations of parts not equal")
	}
	return nil
}

func TestClientConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener3, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer := grpc.NewServer()
	failingServer := grpc.NewServer()
	failingServer2 := grpc.NewServer()

	workingFrameService := &inject.FrameSystemService{}
	failingFrameService := &inject.FrameSystemService{}
	failingFrameService2 := &inject.FrameSystemService{}

	fsConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Parent:      referenceframe.World,
				Translation: spatialmath.Translation{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
		{
			Name: "frame2",
			FrameConfig: &config.Frame{
				Parent:      "frame1",
				Translation: spatialmath.Translation{X: 1, Y: 2, Z: 3},
			},
		},
	}

	workingFrameService.ConfigFunc = func(ctx context.Context) ([]*config.FrameSystemPart, error) {
		return fsConfigs, nil
	}
	configErr := errors.New("failed to retrieve config")
	failingFrameService.ConfigFunc = func(ctx context.Context) ([]*config.FrameSystemPart, error) {
		return nil, configErr
	}

	badFSConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Translation: spatialmath.Translation{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
	}
	failingFrameService2.ConfigFunc = func(ctx context.Context) ([]*config.FrameSystemPart, error) {
		return badFSConfigs, nil
	}

	workingSvc, err := subtype.New(map[resource.Name]interface{}{
		framesystem.Name: workingFrameService,
	})
	test.That(t, err, test.ShouldBeNil)
	failingSvc, err := subtype.New(map[resource.Name]interface{}{
		framesystem.Name: failingFrameService,
	})
	test.That(t, err, test.ShouldBeNil)
	failingSvc2, err := subtype.New(map[resource.Name]interface{}{
		framesystem.Name: failingFrameService2,
	})
	test.That(t, err, test.ShouldBeNil)

	servicepb.RegisterFrameSystemServiceServer(workingServer, framesystem.NewServer(workingSvc))
	servicepb.RegisterFrameSystemServiceServer(failingServer, framesystem.NewServer(failingSvc))
	servicepb.RegisterFrameSystemServiceServer(failingServer2, framesystem.NewServer(failingSvc2))

	go workingServer.Serve(listener1)
	defer workingServer.Stop()

	t.Run("Failing client due to cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = framesystem.NewClient(cancelCtx, framesystem.Name.String(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingFSClient, err := framesystem.NewClient(
		context.Background(), framesystem.Name.String(),
		listener1.Addr().String(), logger, rpc.WithInsecure(),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for working frame service", func(t *testing.T) {
		frameSystemParts, err := workingFSClient.Config(context.Background())
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], frameSystemParts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], frameSystemParts[1])
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("dialed client test frame system for working frame service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := framesystem.NewClientFromConn(context.Background(), conn, "", logger)
		frameSys, err := workingDialedClient.LocalFrameSystem(context.Background(), "", "")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, frameSys, test.ShouldNotBeNil)
		frame1 := frameSys.GetFrame("frame1")
		frame1Offset := frameSys.GetFrame("frame1_offset")
		frame2 := frameSys.GetFrame("frame2")
		frame2Offset := frameSys.GetFrame("frame2_offset")

		resFrame, err := frameSys.Parent(frame2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resFrame, test.ShouldResemble, frame2Offset)
		resFrame, err = frameSys.Parent(frame2Offset)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resFrame, test.ShouldResemble, frame1)
		resFrame, err = frameSys.Parent(frame1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resFrame, test.ShouldResemble, frame1Offset)
		resFrame, err = frameSys.Parent(frame1Offset)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resFrame, test.ShouldResemble, frameSys.World())
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	failingFSClient, err := framesystem.NewClient(
		context.Background(), framesystem.Name.String(),
		listener2.Addr().String(), logger, rpc.WithInsecure(),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for failing frame service", func(t *testing.T) {
		frameSystemParts, err := failingFSClient.Config(context.Background())
		test.That(t, frameSystemParts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("dialed client test frame system for failing frame service with failing config", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener2.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		failingDialedClient := framesystem.NewClientFromConn(context.Background(), conn, "", logger)
		frameSys, err := failingDialedClient.LocalFrameSystem(context.Background(), "", "")
		test.That(t, frameSys, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	go failingServer2.Serve(listener3)
	defer failingServer2.Stop()

	failingFSClient2, err := framesystem.NewClient(
		context.Background(), framesystem.Name.String(),
		listener3.Addr().String(), logger, rpc.WithInsecure(),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test frame system for failing frame service", func(t *testing.T) {
		frameSys, err := failingFSClient2.LocalFrameSystem(context.Background(), "", "")
		test.That(t, frameSys, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
