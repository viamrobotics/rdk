package rtsp

import (
	"log"
	"sync"
	"testing"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage/transform"
)

type serverHandler struct {
	s         *gortsplib.Server
	mutex     sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Print("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Print("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Print("session closed")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if the session is the publisher,
	// close the stream and disconnect any reader.
	if sh.stream != nil && ctx.Session == sh.publisher {
		sh.stream.Close()
		sh.stream = nil
	}
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Print("describe request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	// send medias that are being published to the client
	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	log.Print("announce request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// disconnect existing publisher
	if sh.stream != nil {
		sh.stream.Close()
		sh.publisher.Close()
	}

	// create the stream and save the publisher
	sh.stream = gortsplib.NewServerStream(sh.s, ctx.Description)
	sh.publisher = ctx.Session

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Print("setup request")

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Print("play request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	log.Print("record request")

	// called when receiving a RTP packet
	ctx.Session.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		// route the RTP packet to all readers
		sh.stream.WritePacketRTP(medi, pkt)
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func TestRTSPCamera(t *testing.T) {
	// logger := logging.NewTestLogger(t)
	h := &serverHandler{}
	h.s = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}
	// TODO: Reimplement these tests
	// host := "127.0.0.1"
	// port := "32512"
	// outputURL := fmt.Sprintf("rtsp://%s:%s/mystream", host, port)
	// // set up a simple server, which expects specific requests in a certain order
	// l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	// test.That(t, err, test.ShouldBeNil)
	// defer l.Close()
	// viamutils.PanicCapturingGo(func() {
	// 	nconn, err := l.Accept()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	conx := conn.NewConn(nconn)
	// 	defer nconn.Close()

	// 	// OPTIONS
	// 	req, err := conx.ReadRequest()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, req.Method, test.ShouldEqual, base.Options)
	// 	logger.Debug("in options")
	// 	err = conx.WriteResponse(&base.Response{
	// 		StatusCode: base.StatusOK,
	// 		Header: base.Header{
	// 			"Public": base.HeaderValue{strings.Join([]string{
	// 				string(base.Describe),
	// 				string(base.Setup),
	// 				string(base.Play),
	// 			}, ", ")},
	// 			"Session": base.HeaderValue{"123456"},
	// 		},
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	// DESCRIBE
	// 	req, err = conx.ReadRequest()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, req.Method, test.ShouldEqual, base.Describe)
	// 	logger.Debug("in describe")
	// 	testMJPEG := &media.Media{
	// 		Type:      media.TypeVideo,
	// 		Direction: media.DirectionRecvonly,
	// 		Formats:   []format.Format{&format.MJPEG{}},
	// 	}
	// 	medias := media.Medias{testMJPEG}
	// 	medias.SetControls()
	// 	mediaBytes, err := medias.Marshal(false).Marshal()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	err = conx.WriteResponse(&base.Response{
	// 		StatusCode: base.StatusOK,
	// 		Header: base.Header{
	// 			"Content-Type": base.HeaderValue{"application/sdp"},
	// 			"Session":      base.HeaderValue{"123456"},
	// 			"Content-Base": base.HeaderValue{outputURL},
	// 		},
	// 		Body: mediaBytes,
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	// SETUP
	// 	req, err = conx.ReadRequest()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, req.Method, test.ShouldEqual, base.Setup)
	// 	logger.Debug("in setup")
	// 	var inTH headers.Transport
	// 	err = inTH.Unmarshal(req.Header["Transport"])
	// 	test.That(t, err, test.ShouldBeNil)

	// 	th := headers.Transport{
	// 		Delivery: func() *headers.TransportDelivery {
	// 			v := headers.TransportDeliveryUnicast
	// 			return &v
	// 		}(),
	// 		Protocol:    headers.TransportProtocolUDP,
	// 		ClientPorts: inTH.ClientPorts,
	// 		ServerPorts: &[2]int{34556, 34557},
	// 	}

	// 	err = conx.WriteResponse(&base.Response{
	// 		StatusCode: base.StatusOK,
	// 		Header: base.Header{
	// 			"Session":   base.HeaderValue{"123456"},
	// 			"Transport": th.Marshal(),
	// 		},
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// 	// PLAY
	// 	req, err = conx.ReadRequest()
	// 	test.That(t, err, test.ShouldBeNil)
	// 	test.That(t, req.Method, test.ShouldEqual, base.Play)
	// 	logger.Debug("in play")
	// 	err = conx.WriteResponse(&base.Response{
	// 		StatusCode: base.StatusOK,
	// 		Header: base.Header{
	// 			"Session": base.HeaderValue{"123456"},
	// 		},
	// 	})
	// 	test.That(t, err, test.ShouldBeNil)
	// })

	// rtspConf := &Config{Address: outputURL}
	// timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*10)
	// defer timeoutCancel()
	// rtspCam, err := NewRTSPCamera(timeoutCtx, resource.Name{Name: "foo"}, rtspConf, logger)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, rtspCam.Name().Name, test.ShouldEqual, "foo")
	// // close everything
	// err = rtspCam.Close(context.Background())
	// test.That(t, err, test.ShouldBeNil)
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
		Address: "rtsp://example.com:5000",
		IntrinsicParams: &transform.PinholeCameraIntrinsics{
			Width:  1,
			Height: 2,
			Fx:     3,
			Fy:     4,
			Ppx:    5,
			Ppy:    6,
		},
	}
	_, err = rtspConf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	// no distortion parameters is OK
	rtspConf.DistortionParams = &transform.BrownConrady{}
	test.That(t, err, test.ShouldBeNil)
}
