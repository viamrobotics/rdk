//go:build !no_cgo

package webstream_test

import (
	"context"
	"strings"
	"testing"

	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/x264"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/testutils/robottestutils"
)

// setupRealRobot creates a robot from the input config and starts a WebRTC server with video
// streaming capabilities.
//
//nolint:lll
func setupRealRobot(t *testing.T, robotConfig *config.Config, logger logging.Logger) (context.Context, robot.LocalRobot, string, web.Service) {
	t.Helper()

	ctx := context.Background()
	robot, err := robotimpl.RobotFromConfig(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// We initialize with a stream config such that the stream server is capable of creating video stream and
	// audio stream data.
	webSvc := web.New(robot, logger, web.WithStreamConfig(gostream.StreamConfig{
		AudioEncoderFactory: opus.NewEncoderFactory(),
		VideoEncoderFactory: x264.NewEncoderFactory(),
	}))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = webSvc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	return ctx, robot, addr, webSvc
}

// TestAudioTrackIsNotCreatedForVideoStream asserts that starting a video stream from a camera does
// not create an audio stream. It's not a business requirement that camera streams remain
// silent. Rather there was intentional code for cameras at robot startup to only register
// themselves as video stream sources. But cameras added after robot startup would not behave the
// same. This test asserts both paths to adding a camera agree on whether to create WebRTC audio
// tracks.
func TestAudioTrackIsNotCreatedForVideoStream(t *testing.T) {
	// Pion has a bug where `PeerConnection.AddTrack` is non-deterministic w.r.t creating a media
	// track in the SDP due to races in their renegotiation piggy-backing algorithm. This is easy to
	// reproduce by runnnig the test in a loop.
	t.Skip("RSDK-7492")
	logger := logging.NewTestLogger(t).Sublogger("TestWebReconfigure")

	origCfg := &config.Config{Components: []resource.Config{
		{
			Name:  "origCamera",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Animated: true,
				Width:    100,
				Height:   50,
			},
		},
	}}

	// Create a robot with a single fake camera.
	ctx, robot, addr, webSvc := setupRealRobot(t, origCfg, logger)

	defer robot.Close(ctx)
	defer webSvc.Close(ctx)

	// Create a client connection to the robot. Disable direct GRPC to force a WebRTC
	// connection. Fail if a WebRTC connection cannot be made.
	conn, err := rgrpc.Dial(context.Background(), addr, logger.Sublogger("TestDial"), rpc.WithDisableDirectGRPC())
	//nolint
	defer conn.Close()
	test.That(t, err, test.ShouldBeNil)

	// Get a handle on the camera client named in the robot config.
	cam, err := camera.NewClientFromConn(context.Background(), conn, "", camera.Named("origCamera"), logger)
	test.That(t, err, test.ShouldBeNil)
	defer cam.Close(ctx)

	// Test that getting a single image succeeds.
	_, _, err = cam.Images(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Create a stream client. Listing the streams should give back a single stream named `origCamera`;
	// after our component name.
	livestreamClient := streampb.NewStreamServiceClient(conn)
	listResp, err := livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, listResp.Names, test.ShouldResemble, []string{"origCamera"})

	// Assert there are no video/audio tracks on the peer connection.
	test.That(t, conn.PeerConn().LocalDescription().SDP, test.ShouldNotContainSubstring, "m=video")
	test.That(t, conn.PeerConn().LocalDescription().SDP, test.ShouldNotContainSubstring, "m=audio")

	logger.Info("Adding stream `origCamera`")
	// Call the `AddStream` endpoint to request that the server start streaming video backed by the
	// robot component named `origCamera`. This should result in a non-error response.
	_, err = livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{
		Name: "origCamera",
	})
	test.That(t, err, test.ShouldBeNil)

	// Upon receiving the `AddStreamRequest`, the robot server will renegotiate the PeerConnection
	// to add a video track. This happens asynchronously. Wait for the video track to appear.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, conn.PeerConn().CurrentLocalDescription().SDP, test.ShouldContainSubstring, "m=video")
	})
	// Until we support sending a camera's video and audio data, sending an `AddStreamRequest` for a
	// camera should only create a video track. Assert the audio track does not exist.
	test.That(t, conn.PeerConn().CurrentLocalDescription().SDP, test.ShouldNotContainSubstring, "m=audio")

	// Dan: While testing adding a new camera + stream, it made sense to me to call `RemoveStream`
	// on `origCamera`. However, I couldn't assert any observable state change on the client
	// PeerConnection. Specifically, when adding the first stream we can see the client peer
	// connection object go from not having any video tracks to having one video track with the
	// property of `a=recvonly`. The server similarly creates a video track, but it takes on the
	// property of `a=sendrecv`.
	//
	// When removing the stream, the server keeps the video track, but its state is now in
	// `a=recvonly`. Which I suppose is reasonable. And while I am experimentally observing that
	// this does result in an "offer" being presented to the client, the client ultimately does not
	// change its video track state. The client's video track remains in the SDP and it stays in the
	// `a=recvonly` state. I believe I've observed in chrome the video track gets moved into an
	// `a=inactive` state.
	//
	// Given the client SDP seems to remain logically the same, I've commented out this part of the
	// test.
	//
	// logger.Info("Removing stream `origCamera`")
	// _, err = livestreamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{
	//  	Name: "origCamera",
	// })

	logger.Info("Reconfiguring with `newCamera`")
	// Update the config to add a `newCamera`.
	origCfg.Components = append(origCfg.Components, resource.Config{
		Name:  "newCamera",
		API:   resource.NewAPI("rdk", "component", "camera"),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fake.Config{
			Animated: true,
			Width:    100,
			Height:   50,
		},
	})
	// And reconfigure the robot which will create a `newCamera` component.
	robot.Reconfigure(ctx, origCfg)

	// Reconfigure the websvc to update the available video streams. Such that `newCamera` is picked
	// up.
	//
	// Dan: It's unclear to me if this is the intended way to do this. As signified by the nil/empty
	//      inputs.
	webSvc.Reconfigure(ctx, nil, resource.Config{})

	// Listing the streams now should return both `origCamera` and `newCamera`.
	listResp, err = livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, listResp.Names, test.ShouldResemble, []string{"origCamera", "newCamera"})

	logger.Info("Adding stream `newCamera`")
	// Call the `AddStream` endpoint to request that the server start streaming video backed by the
	// robot component named `newCamera`. This should result in a non-error response.
	_, err = livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{
		Name: "newCamera",
	})
	test.That(t, err, test.ShouldBeNil)

	// Upon receiving the `AddStreamRequest`, the robot server will renegotiate the PeerConnection
	// to add a video track. This happens asynchronously. Wait for the video track to appear.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
		test.That(tb, videoCnt, test.ShouldEqual, 2)
	})

	// Until we support sending a camera's video and audio data, sending an `AddStreamRequest` for a
	// camera should only create a video track. Assert the audio track does not exist.
	test.That(t, conn.PeerConn().CurrentLocalDescription().SDP, test.ShouldNotContainSubstring, "m=audio")
}
