package server_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/config"
	grpcserver "go.viam.com/rdk/grpc/server"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.RobotServiceServer, *inject.Robot) {
	injectRobot := &inject.Robot{}
	return grpcserver.New(injectRobot), injectRobot
}

func TestServer(t *testing.T) {
	t.Run("Config", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
			return nil, err1
		}
		_, err := server.Config(context.Background(), &pb.ConfigRequest{})
		test.That(t, err, test.ShouldEqual, err1)

		cfg := config.Config{
			Components: []config.Component{
				{
					Name: "a",
					Type: config.ComponentTypeArm,
					Frame: &config.Frame{
						Parent: "b",
					},
				},
			},
		}
		injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
			return &cfg, nil
		}
		statusResp, err := server.Config(context.Background(), &pb.ConfigRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statusResp.Components), test.ShouldEqual, len(cfg.Components))
		test.That(t, statusResp.Components[0].Name, test.ShouldEqual, cfg.Components[0].Name)
		test.That(t, statusResp.Components[0].Parent, test.ShouldEqual, cfg.Components[0].Frame.Parent)
		test.That(t, statusResp.Components[0].Type, test.ShouldResemble, string(cfg.Components[0].Type))
	})

	t.Run("DoAction", func(t *testing.T) {
		server, injectRobot := newServer()
		_, err := server.DoAction(context.Background(), &pb.DoActionRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown action")

		actionName := utils.RandomAlphaString(5)
		called := make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)

		actionName = utils.RandomAlphaString(5)
		called = make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			go utils.TryClose(context.Background(), server)
			<-ctx.Done()
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)
	})
}
