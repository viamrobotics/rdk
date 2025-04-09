//go:build !no_cgo

package webstream_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pion/rtp"
	viamwebrtc "github.com/viamrobotics/webrtc/v3"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/components/camera/rtppassthrough"
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
	webstream "go.viam.com/rdk/robot/web/stream"
	"go.viam.com/rdk/testutils/robottestutils"
)

// streamServerGetter is a test-only interface to access the stream server.
type streamServerGetter interface {
	GetStreamServer() *webstream.Server
}

// setupRealRobot creates a robot from the input config and starts a WebRTC server with video
// streaming capabilities.
//
//nolint:lll
func setupRealRobot(t *testing.T, robotConfig *config.Config, logger logging.Logger) (context.Context, robot.LocalRobot, string, web.Service, *webstream.Server) {
	t.Helper()

	ctx := context.Background()
	robot, err := robotimpl.RobotFromConfig(ctx, robotConfig, nil, logger)
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

	// Make type assertion to access the stream server.
	getter, ok := webSvc.(streamServerGetter)
	test.That(t, ok, test.ShouldBeTrue)
	streamServer := getter.GetStreamServer()

	return ctx, robot, addr, webSvc, streamServer
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
	ctx, robot, addr, webSvc, streamServer := setupRealRobot(t, origCfg, logger)
	defer robot.Close(ctx)
	defer webSvc.Close(ctx)
	if streamServer == nil {
		t.Fatal("stream server is nil")
	}

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

func TestGetStreamOptions(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger("GetStreamOptions")
	// Create a robot with several fake cameras of common resolutions.
	// Fake cameras with a Model attribute will use Properties to
	// determine source resolution. Fake cameras without a Model
	// attribute will sample a frame to determine source resolution.
	origCfg := &config.Config{Components: []resource.Config{
		// 480p
		{
			Name:  "fake-cam-0-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  640,
				Height: 480,
			},
		},
		{
			Name:  "fake-cam-0-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  640,
				Height: 480,
				Model:  true,
			},
		},
		// 720p
		{
			Name:  "fake-cam-1-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  1280,
				Height: 720,
			},
		},
		{
			Name:  "fake-cam-1-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  1280,
				Height: 720,
				Model:  true,
			},
		},
		// 1080p
		{
			Name:  "fake-cam-2-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Animated: true,
				Width:    1920,
				Height:   1080,
			},
		},
		{
			Name:  "fake-cam-2-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Animated: true,
				Width:    1920,
				Height:   1080,
				Model:    true,
			},
		},
		// Really small resolution
		{
			Name:  "fake-cam-3-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  2,
				Height: 2,
			},
		},
		{
			Name:  "fake-cam-3-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  2,
				Height: 2,
				Model:  true,
			},
		},
		// Odd resolutions gets rounded to nearest even resolution
		// 100x100 downscaled to 25*25 turns into 24*24
		{
			Name:  "fake-cam-4-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  100,
				Height: 100,
			},
		},
		{
			Name:  "fake-cam-4-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  100,
				Height: 100,
				Model:  true,
			},
		},
	}}

	ctx, robot, addr, webSvc, _ := setupRealRobot(t, origCfg, logger)
	defer robot.Close(ctx)
	defer webSvc.Close(ctx)
	conn, err := rgrpc.Dial(context.Background(), addr, logger.Sublogger("TestDial"), rpc.WithDisableDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	livestreamClient := streampb.NewStreamServiceClient(conn)
	listResp, err := livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(listResp.Names), test.ShouldEqual, 10)

	streamOptionsResp, err := livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, streamOptionsResp, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "name")

	streamOptionsResp, err = livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{
		Name: "invalid-name",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, streamOptionsResp, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// Sanity check that we get valid stream options for both properties and sampling.
	streamOptionsResp, err = livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{
		Name: "fake-cam-1-0",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, streamOptionsResp, test.ShouldNotBeNil)
	test.That(t, len(streamOptionsResp.Resolutions), test.ShouldEqual, 5)
	streamOptionsResp, err = livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{
		Name: "fake-cam-1-1",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, streamOptionsResp, test.ShouldNotBeNil)
	test.That(t, len(streamOptionsResp.Resolutions), test.ShouldEqual, 5)

	testGetStreamOptions := func(name string, expectedResolutions []webstream.Resolution) {
		streamOptionsResp, err := livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{
			Name: name,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, streamOptionsResp, test.ShouldNotBeNil)
		test.That(t, len(streamOptionsResp.Resolutions), test.ShouldEqual, len(expectedResolutions))
		for i, expected := range expectedResolutions {
			test.That(t, streamOptionsResp.Resolutions[i].Width, test.ShouldEqual, expected.Width)
			test.That(t, streamOptionsResp.Resolutions[i].Height, test.ShouldEqual, expected.Height)
		}
	}

	// Define expected resolutions based on camera resolutions
	resolutionsMap := map[string][]webstream.Resolution{
		"fake-cam-0-0": webstream.GenerateResolutions(640, 480, logger),
		"fake-cam-0-1": webstream.GenerateResolutions(640, 480, logger),
		"fake-cam-1-0": webstream.GenerateResolutions(1280, 720, logger),
		"fake-cam-1-1": webstream.GenerateResolutions(1280, 720, logger),
		"fake-cam-2-0": webstream.GenerateResolutions(1920, 1080, logger),
		"fake-cam-2-1": webstream.GenerateResolutions(1920, 1080, logger),
		"fake-cam-3-0": webstream.GenerateResolutions(2, 2, logger),
		"fake-cam-3-1": webstream.GenerateResolutions(2, 2, logger),
		"fake-cam-4-0": webstream.GenerateResolutions(100, 100, logger),
		"fake-cam-4-1": webstream.GenerateResolutions(100, 100, logger),
	}

	// Test each camera
	for name, expectedResolutions := range resolutionsMap {
		testGetStreamOptions(name, expectedResolutions)
	}
}

func TestSetStreamOptions(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger("TestSetStreamOptions")
	origCfg := &config.Config{Components: []resource.Config{
		// 480p
		{
			Name:  "fake-cam-0-0",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:  640,
				Height: 480,
			},
		},
		{
			Name:  "fake-cam-0-1",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:          640,
				Height:         480,
				RTPPassthrough: true,
			},
		},
	}}

	ctx, robot, addr, webSvc, _ := setupRealRobot(t, origCfg, logger)
	defer robot.Close(ctx)
	defer webSvc.Close(ctx)
	conn, err := rgrpc.Dial(context.Background(), addr, logger.Sublogger("TestDial"), rpc.WithDisableDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	livestreamClient := streampb.NewStreamServiceClient(conn)
	listResp, err := livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(listResp.Names), test.ShouldEqual, 2)

	t.Run("GetStreamOptions", func(t *testing.T) {
		getStreamOptionsResp, err := livestreamClient.GetStreamOptions(ctx, &streampb.GetStreamOptionsRequest{
			Name: "fake-cam-0-0",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, getStreamOptionsResp, test.ShouldNotBeNil)
		test.That(t, len(getStreamOptionsResp.Resolutions), test.ShouldEqual, 5)
	})

	t.Run("SetStreamOptions with invalid stream name", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name:       "invalid-name",
			Resolution: &streampb.Resolution{Width: 320, Height: 240},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	})

	t.Run("SetStreamOptions without name", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "name")
	})

	// Test setting stream options with invalid resolution (0x0)
	t.Run("SetStreamOptions with invalid resolution", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name:       "fake-cam-0-0",
			Resolution: &streampb.Resolution{Width: 0, Height: 0},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid resolution")
	})

	// Test setting stream options with an odd resolution
	t.Run("SetStreamOptions with odd resolution", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name:       "fake-cam-0-0",
			Resolution: &streampb.Resolution{Width: 25, Height: 25},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid resolution")
	})

	t.Run("AddStream creates video track", func(t *testing.T) {
		res, err := livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{
			Name: "fake-cam-0-0",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res, test.ShouldNotBeNil)
		logger.Info("Checking video track is created")
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			logger.Info(conn.PeerConn().CurrentLocalDescription().SDP)
			videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
			test.That(tb, videoCnt, test.ShouldEqual, 1)
		})
	})

	t.Run("SetStreamOptions with valid resolution", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name:       "fake-cam-0-0",
			Resolution: &streampb.Resolution{Width: 320, Height: 240},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldNotBeNil)
		// Make sure that video strack is still alive through the peer connection
		videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
		test.That(t, videoCnt, test.ShouldEqual, 1)
	})

	t.Run("SetStreamOptions without resolution field to reset to the original resolution", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "fake-cam-0-0",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldNotBeNil)
		videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
		test.That(t, videoCnt, test.ShouldEqual, 1)
	})

	t.Run("AddStream with RTPPassthrough enabled", func(t *testing.T) {
		res, err := livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{
			Name: "fake-cam-0-1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res, test.ShouldNotBeNil)
		logger.Info(conn.PeerConn().CurrentLocalDescription().SDP)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
			test.That(tb, videoCnt, test.ShouldEqual, 2)
		})
	})

	t.Run("SetStreamOptions with RTPPassthrough enabled", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name:       "fake-cam-0-1",
			Resolution: &streampb.Resolution{Width: 320, Height: 240},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldNotBeNil)
		videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
		test.That(t, videoCnt, test.ShouldEqual, 2)
	})

	t.Run("SetStreamOptions reset to original resolution when RTPPassthrough is enabled", func(t *testing.T) {
		setStreamOptionsResp, err := livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "fake-cam-0-1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, setStreamOptionsResp, test.ShouldNotBeNil)
		videoCnt := strings.Count(conn.PeerConn().CurrentLocalDescription().SDP, "m=video")
		test.That(t, videoCnt, test.ShouldEqual, 2)
	})

	t.Run("RemoveStream", func(t *testing.T) {
		removeRes, err := livestreamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{
			Name: "fake-cam-0-0",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, removeRes, test.ShouldNotBeNil)
		removeRes, err = livestreamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{
			Name: "fake-cam-0-1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, removeRes, test.ShouldNotBeNil)
	})
}

// TestStreamMediaBehavior verifies that WebRTC media streams survive reconfigures and
// that the resulting images are in the expected format from stream options handling and resets.
func TestStreamMediaBehavior(t *testing.T) {
	logger := logging.NewTestLogger(t).Sublogger("TestStreamMediaBehavior")
	origCfg := &config.Config{Components: []resource.Config{
		{
			Name:  "test-camera",
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{
				Width:          1280,
				Height:         720,
				RTPPassthrough: true,
				Model:          true,
			},
		},
	}}
	// Pass a blank logger to setupRealRobot to prevent race conditions with test logger usage in background goroutines.
	ctx, robot, addr, webSvc, streamServer := setupRealRobot(t, origCfg, logging.NewBlankLogger(""))
	defer robot.Close(ctx)
	defer webSvc.Close(ctx)

	if streamServer == nil {
		t.Fatal("stream server is nil")
	}

	conn, err := rgrpc.Dial(context.Background(), addr, logger.Sublogger("TestDial"), rpc.WithDisableDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	camResource, err := robot.ResourceByName(camera.Named("test-camera"))
	test.That(t, err, test.ShouldBeNil)
	camServer, ok := camResource.(camera.Camera)
	test.That(t, ok, test.ShouldBeTrue)

	// Cast the server-side resource to rtppassthrough.Source to set up for a subscription
	rtpSource, ok := camServer.(rtppassthrough.Source)
	test.That(t, ok, test.ShouldBeTrue)

	// Create stream client (using the connection)
	livestreamClient := streampb.NewStreamServiceClient(conn)

	// Verify stream is available
	listResp, err := livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, listResp.Names, test.ShouldResemble, []string{"test-camera"})

	// Add test-camera stream to client
	_, err = livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: "test-camera"})
	test.That(t, err, test.ShouldBeNil)

	// Wait for video track to be added
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		// Handle potential nil description if connection closes early
		desc := conn.PeerConn().CurrentLocalDescription()
		if desc == nil {
			tb.Log("Peer connection local description is nil, likely closed early")
			return
		}
		test.That(tb, desc.SDP, test.ShouldContainSubstring, "m=video")
		// Check that H264 is negotiated, regardless of the dynamic payload type
		test.That(tb, desc.SDP, test.ShouldContainSubstring, "H264/90000")
	})

	// Wait for connection to be stable again
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		state := conn.PeerConn().ConnectionState()
		test.That(tb, state, test.ShouldEqual, viamwebrtc.PeerConnectionStateConnected)
		iceState := conn.PeerConn().ICEConnectionState()
		test.That(tb, iceState, test.ShouldEqual, viamwebrtc.ICEConnectionStateConnected)
	})

	pktsChan := make(chan []*rtp.Packet, 10)
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	sub, err := rtpSource.SubscribeRTP(subCtx, 512, func(pkts []*rtp.Packet) {
		select {
		case <-subCtx.Done():
			return
		default:
		}

		if len(pkts) > 0 {
			now := time.Now()
			logger.Debugf("Callback received %d packets at %s", len(pkts), now.Format(time.RFC3339Nano))
			select {
			case pktsChan <- pkts:
			default:
				logger.Debug("Packet channel full, dropping packet")
			}
		}
	})
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		logger.Debug("Unsubscribing from RTP source")
		// Use the original test context for unsubscribe to ensure cleanup proceeds
		// even if subCtx was cancelled.
		unsubscribeErr := rtpSource.Unsubscribe(ctx, sub.ID)
		test.That(t, unsubscribeErr, test.ShouldBeNil)
		logger.Debug("Unsubscribed from RTP source")
	}()

	// Wait for the first packet
	timeout := time.After(15 * time.Second)
	var firstPkts []*rtp.Packet
	select {
	case p, ok := <-pktsChan:
		if !ok {
			t.Fatal("pktsChan was closed unexpectedly before first packet")
		}
		if p == nil {
			t.Fatal("received nil packet slice from pktsChan for first packet")
		}
		if len(p) == 0 {
			t.Fatal("received empty packet slice from pktsChan for first packet")
		}
		firstPkts = p // assign received packets
		logger.Infof("Received first packet(s): count=%d", len(firstPkts))
	case <-timeout:
		connState := conn.PeerConn().ConnectionState()
		iceState := conn.PeerConn().ICEConnectionState()
		t.Fatalf("timeout waiting for the first RTP packet (conn state: %s, ICE state: %s)", connState, iceState)
	case <-ctx.Done(): // ctx is the main test context
		t.Fatal("test context cancelled while waiting for the first packet")
	}

	// Verify first packet
	test.That(t, len(firstPkts), test.ShouldBeGreaterThan, 0)
	pkt := firstPkts[0]
	test.That(t, pkt.PayloadType, test.ShouldEqual, 96)         // H264
	test.That(t, pkt.Version, test.ShouldEqual, 2)              // RTP version
	test.That(t, len(pkt.Payload), test.ShouldBeGreaterThan, 0) // Verify packet has payload
	logger.Infof("First packet verified: payload size=%d bytes", len(pkt.Payload))

	logger.Info("Initial packet verified successfully.")
	initialPktLen := len(pkt.Payload) // Store initial packet length

	// --- Test Reconfiguration Survival ---
	t.Run("Test reconfiguration survival", func(t *testing.T) {
		logger.Info("Performing robot reconfiguration...")
		newCfg := origCfg
		robot.Reconfigure(ctx, newCfg)
		webSvc.Reconfigure(ctx, nil, resource.Config{})
		logger.Info("Reconfiguration complete.")

		// Verify the packets continue flowing after reconfigure
		timeout = time.After(5 * time.Second)
		select {
		case pkts := <-pktsChan:
			test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
			logger.Infof("Received %d packets after reconfiguration", len(pkts))
		case <-timeout:
			connState := conn.PeerConn().ConnectionState()
			iceState := conn.PeerConn().ICEConnectionState()
			t.Fatalf("timeout waiting for RTP packets after reconfiguration (conn state: %s, ICE state: %s)", connState, iceState)
		case <-subCtx.Done():
			t.Fatal("Subscription context cancelled unexpectedly during reconfiguration test")
		}
	})

	// Test stream resize with multiple resolutions
	t.Run("Test multiple resolution changes", func(t *testing.T) {
		// Test 640x360
		_, err = livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "test-camera",
			Resolution: &streampb.Resolution{
				Width:  640,
				Height: 360,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		logger.Info("Set resolution to 640x360")

		// Verify source dimensions changed via server-side inspection
		vs, ok := streamServer.GetVideoSourceForTest("test-camera")
		test.That(t, ok, test.ShouldBeTrue)
		mediaProps, err := vs.MediaProperties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mediaProps.Width, test.ShouldEqual, 640)
		test.That(t, mediaProps.Height, test.ShouldEqual, 360)

		// Test 320x240
		_, err = livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "test-camera",
			Resolution: &streampb.Resolution{
				Width:  320,
				Height: 240,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		logger.Info("Set resolution to 320x240")

		// Verify source dimensions changed
		vs, ok = streamServer.GetVideoSourceForTest("test-camera")
		test.That(t, ok, test.ShouldBeTrue)
		mediaProps, err = vs.MediaProperties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mediaProps.Width, test.ShouldEqual, 320)
		test.That(t, mediaProps.Height, test.ShouldEqual, 240)

		// Test invalid resolution
		_, err = livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "test-camera",
			Resolution: &streampb.Resolution{
				Width:  0,
				Height: 0,
			},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid resolution")
		logger.Info("Verified invalid resolution error handling")

		// Reset to original resolution - Should switch back to passthrough
		_, err = livestreamClient.SetStreamOptions(ctx, &streampb.SetStreamOptionsRequest{
			Name: "test-camera",
			Resolution: &streampb.Resolution{
				Width:  1280,
				Height: 720,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		logger.Info("Reset to original resolution")

		// Verify source dimensions reset.
		// NOTE: We use a workaround here (direct resource lookup) because checking the
		// HotSwappableVideoSource's MediaProperties immediately after the swap in resetVideoSource
		// was observed to return stale data in previous debugging sessions.
		vs, ok = streamServer.GetVideoSourceForTest("test-camera") // Swapper check is unreliable here
		test.That(t, ok, test.ShouldBeTrue)
		mediaProps, err = vs.MediaProperties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mediaProps.Width, test.ShouldEqual, 1280)
		test.That(t, mediaProps.Height, test.ShouldEqual, 720)

		// Wait for switch back to passthrough and packets to resume on pktsChan
		resumeTimeout := time.After(5 * time.Second)
		var resetPkts []*rtp.Packet
		select {
		case p, ok := <-pktsChan:
			if !ok {
				t.Fatal("pktsChan closed unexpectedly while waiting for reset packet")
			}
			if len(p) == 0 {
				t.Fatal("Received empty packet slice after reset")
			}
			resetPkts = p
			logger.Infof("Received packet batch after reset (count=%d)", len(resetPkts))
		case <-resumeTimeout:
			connState := conn.PeerConn().ConnectionState()
			iceState := conn.PeerConn().ICEConnectionState()
			t.Fatalf("timeout waiting for RTP packets after reset (conn state: %s, ICE state: %s)", connState, iceState)
		case <-subCtx.Done():
			t.Fatal("Subscription context cancelled unexpectedly during reset resume check")
		}
		resetPkt := resetPkts[0]
		test.That(t, len(resetPkt.Payload), test.ShouldEqual, initialPktLen)
		logger.Infof("Verified packet payload length reverted to initial (%d) after reset", initialPktLen)
	})

	t.Run("Test stream removal and re-addition", func(t *testing.T) {
		_, err = livestreamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{Name: "test-camera"})
		test.That(t, err, test.ShouldBeNil)
		logger.Info("Removed stream")

		// Verify stream is still listed after RemoveStream (peer-specific removal)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			listResp, err := livestreamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
			test.That(tb, err, test.ShouldBeNil)

			// Log the current stream list for debugging
			logger.Debugf("Current stream list after RemoveStream: %v", listResp.Names)

			// Check that test-camera IS STILL in the list because RemoveStream is peer-specific, not global
			test.That(tb, listResp.Names, test.ShouldHaveLength, 1)
			test.That(tb, listResp.Names[0], test.ShouldEqual, "test-camera")
		})
		logger.Info("Verified stream is still listed after peer-specific RemoveStream")

		// Re-add stream
		_, err = livestreamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: "test-camera"})
		test.That(t, err, test.ShouldBeNil)
		logger.Info("Re-added stream")

		// Wait for video track to be re-added
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			desc := conn.PeerConn().CurrentLocalDescription()
			if desc == nil {
				tb.Log("Peer connection local description is nil, likely closed early")
				return
			}
			test.That(tb, desc.SDP, test.ShouldContainSubstring, "m=video")
			test.That(tb, desc.SDP, test.ShouldContainSubstring, "H264/90000")
		})

		// Wait for connection to be stable again
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			state := conn.PeerConn().ConnectionState()
			test.That(tb, state, test.ShouldEqual, viamwebrtc.PeerConnectionStateConnected)
			iceState := conn.PeerConn().ICEConnectionState()
			test.That(tb, iceState, test.ShouldEqual, viamwebrtc.ICEConnectionStateConnected)
		})

		// Verify packets continue flowing after re-addition
		timeout = time.After(5 * time.Second)
		select {
		case pkts := <-pktsChan:
			test.That(t, len(pkts), test.ShouldBeGreaterThan, 0)
			logger.Infof("Received %d packets after stream re-addition", len(pkts))
		case <-timeout:
			connState := conn.PeerConn().ConnectionState()
			iceState := conn.PeerConn().ICEConnectionState()
			t.Fatalf("timeout waiting for RTP packets after stream re-addition (conn state: %s, ICE state: %s)", connState, iceState)
		}
	})
}
