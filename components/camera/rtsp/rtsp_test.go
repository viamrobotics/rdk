package rtsp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/conn"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/edaniels/golog"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

func TestRTSPCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	host := "127.0.0.1"
	port := "32512"
	outputURL := fmt.Sprintf("rtsp://%s:%s/mystream", host, port)
	// set up a simple server, which expects specific requests in a certain order
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	test.That(t, err, test.ShouldBeNil)
	defer l.Close()
	viamutils.PanicCapturingGo(func() {
		nconn, err := l.Accept()
		test.That(t, err, test.ShouldBeNil)
		conx := conn.NewConn(nconn)
		defer nconn.Close()

		// OPTIONS
		req, err := conx.ReadRequest()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, req.Method, test.ShouldEqual, base.Options)
		logger.Debug("in options")
		err = conx.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
				"Session": base.HeaderValue{"123456"},
			},
		})
		test.That(t, err, test.ShouldBeNil)
		// DESCRIBE
		req, err = conx.ReadRequest()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, req.Method, test.ShouldEqual, base.Describe)
		logger.Debug("in describe")
		testMJPEG := &media.Media{
			Type:      media.TypeVideo,
			Direction: media.DirectionRecvonly,
			Formats:   []format.Format{&format.MJPEG{}},
		}
		medias := media.Medias{testMJPEG}
		medias.SetControls()
		mediaBytes, err := medias.Marshal(false).Marshal()
		test.That(t, err, test.ShouldBeNil)
		err = conx.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Session":      base.HeaderValue{"123456"},
				"Content-Base": base.HeaderValue{outputURL},
			},
			Body: mediaBytes,
		})
		test.That(t, err, test.ShouldBeNil)
		// SETUP
		req, err = conx.ReadRequest()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, req.Method, test.ShouldEqual, base.Setup)
		logger.Debug("in setup")
		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		test.That(t, err, test.ShouldBeNil)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: inTH.ClientPorts,
			ServerPorts: &[2]int{34556, 34557},
		}

		err = conx.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session":   base.HeaderValue{"123456"},
				"Transport": th.Marshal(),
			},
		})
		test.That(t, err, test.ShouldBeNil)
		// PLAY
		req, err = conx.ReadRequest()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, req.Method, test.ShouldEqual, base.Play)
		logger.Debug("in play")
		err = conx.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"123456"},
			},
		})
		test.That(t, err, test.ShouldBeNil)
	})

	rtspConf := &Config{Address: outputURL}
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*10)
	defer timeoutCancel()
	rtspCam, err := NewRTSPCamera(timeoutCtx, resource.Config{Name: "foo"}, rtspConf, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rtspCam.Name().Name, test.ShouldEqual, "foo")
	// close everything
	err = rtspCam.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestRTSPConfig(t *testing.T) {
	// success
	rtspConf := &Config{Address: "rtsp://example.com:5000"}
	_, err := rtspConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	// badly formatted rtsp address
	rtspConf = &Config{Address: "http://example.com"}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported scheme")
	// bad intrinsic parameters
	rtspConf = &Config{
		Address:         "rtsp://example.com:5000",
		IntrinsicParams: &transform.PinholeCameraIntrinsics{},
	}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	// good intrinsic parameters
	rtspConf = &Config{
		Address:         "rtsp://example.com:5000",
		IntrinsicParams: &transform.PinholeCameraIntrinsics{1, 2, 3, 4, 5, 6},
	}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	// no distortion parameters is OK
	rtspConf.DistortionParams = &transform.BrownConrady{}
	test.That(t, err, test.ShouldBeNil)
}
