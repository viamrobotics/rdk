package rtsp

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

func TestRTSPCamera(t *testing.T) {
	logger := logging.NewTestLogger(t)
	bURL, err := base.ParseURL("rtsp://127.0.0.1:32512")
	test.That(t, err, test.ShouldBeNil)

	t.Run("MJPEG", func(t *testing.T) {
		var forma = &format.MJPEG{}
		red := color.RGBA{255, 0, 0, 0}
		t.Run("init", func(t *testing.T) {
			h := &serverHandler{
				media: &description.Media{
					Type:    description.MediaTypeVideo,
					Formats: []format.Format{forma},
				},
				OnConnOpenFunc: func(ctx *gortsplib.ServerHandlerOnConnOpenCtx, sh *serverHandler) {
					logger.Debug("OnConnOpenFunc")
				},
				OnConnCloseFunc: func(ctx *gortsplib.ServerHandlerOnConnCloseCtx, sh *serverHandler) {
					logger.Debug("OnConnCloseFunc")
				},
				OnSessionOpenFunc: func(ctx *gortsplib.ServerHandlerOnSessionOpenCtx, sh *serverHandler) {
					logger.Debug("OnSessionOpenFunc")
				},
				OnSessionCloseFunc: func(ctx *gortsplib.ServerHandlerOnSessionCloseCtx, sh *serverHandler) {
					logger.Debug("OnSessionCloseFunc")
					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream.Close()
				},
				OnDescribeFunc: func(ctx *gortsplib.ServerHandlerOnDescribeCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnDescribeFunc")

					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream = gortsplib.NewServerStream(sh.s, &description.Session{
						BaseURL: bURL,
						Title:   "123456",
						Medias:  []*description.Media{sh.media},
					})
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnAnnounceFunc: func(ctx *gortsplib.ServerHandlerOnAnnounceCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnAnnounceFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
				OnSetupFunc: func(ctx *gortsplib.ServerHandlerOnSetupCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnSetupFunc")
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnPlayFunc: func(ctx *gortsplib.ServerHandlerOnPlayCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnPlayFunc")
					return &base.Response{StatusCode: base.StatusOK}, nil
				},
				OnRecordFunc: func(ctx *gortsplib.ServerHandlerOnRecordCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnRecordFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
			}
			defer h.wg.Wait()

			h.s = &gortsplib.Server{
				Handler:     h,
				RTSPAddress: "127.0.0.1:32512",
			}
			test.That(t, h.s.Start(), test.ShouldBeNil)
			rtspConf := &Config{Address: "rtsp://" + h.s.RTSPAddress}
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer timeoutCancel()
			rtspCam, err := NewRTSPCamera(timeoutCtx, resource.Name{Name: "foo"}, rtspConf, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rtspCam.Name().Name, test.ShouldEqual, "foo")
			// close everything
			err = rtspCam.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)
			h.s.Close()
			test.That(t, h.s.Wait(), test.ShouldBeError, errors.New("terminated"))
		})

		t.Run("GetImage", func(t *testing.T) {
			h := &serverHandler{
				media: &description.Media{
					Type:    description.MediaTypeVideo,
					Formats: []format.Format{forma},
				},
				OnConnOpenFunc: func(ctx *gortsplib.ServerHandlerOnConnOpenCtx, sh *serverHandler) {
					logger.Debug("OnConnOpenFunc")
				},
				OnConnCloseFunc: func(ctx *gortsplib.ServerHandlerOnConnCloseCtx, sh *serverHandler) {
					logger.Debug("OnConnCloseFunc")
				},
				OnSessionOpenFunc: func(ctx *gortsplib.ServerHandlerOnSessionOpenCtx, sh *serverHandler) {
					logger.Debug("OnSessionOpenFunc")
				},
				OnSessionCloseFunc: func(ctx *gortsplib.ServerHandlerOnSessionCloseCtx, sh *serverHandler) {
					logger.Debug("OnSessionCloseFunc")
					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream.Close()
				},
				OnDescribeFunc: func(ctx *gortsplib.ServerHandlerOnDescribeCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnDescribeFunc")

					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream = gortsplib.NewServerStream(sh.s, &description.Session{
						BaseURL: bURL,
						Title:   "123456",
						Medias:  []*description.Media{sh.media},
					})
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnAnnounceFunc: func(ctx *gortsplib.ServerHandlerOnAnnounceCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnAnnounceFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
				OnSetupFunc: func(ctx *gortsplib.ServerHandlerOnSetupCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnSetupFunc")
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnPlayFunc: func(ctx *gortsplib.ServerHandlerOnPlayCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnPlayFunc")
					sh.wg.Add(1)
					utils.ManagedGo(func() {
						rtpEnc, err := forma.CreateEncoder()
						if err != nil {
							panic(err)
						}
						rtpTime := &rtptime.Encoder{ClockRate: forma.ClockRate()}
						err = rtpTime.Initialize()
						if err != nil {
							panic(err)
						}
						start := time.Now()

						// setup a ticker to sleep between frames
						ticker := time.NewTicker(200 * time.Millisecond)
						defer ticker.Stop()
						image := image.NewRGBA(image.Rect(0, 0, 640, 480))

						// fill the image
						for y := 0; y < image.Rect.Dy(); y++ {
							for x := 0; x < image.Rect.Dx(); x++ {
								image.SetRGBA(x, y, red)
							}
						}

						// encode the image with JPEG
						var buf bytes.Buffer
						err = jpeg.Encode(&buf, image, &jpeg.Options{Quality: 80})
						if err != nil {
							panic(err)
						}

						for range ticker.C {
							sh.mu.Lock()
							if sh.stream == nil {
								sh.mu.Unlock()
								return
							}
							sh.mu.Unlock()
							// generate RTP/M-JPEG packets from the JPEG image
							pkts, err := rtpEnc.Encode(buf.Bytes())
							if err != nil {
								panic(err)
							}

							// get current timestamp
							ts := rtpTime.Encode(time.Since(start))

							// write packets to the server
							for _, pkt := range pkts {
								pkt.Timestamp = ts

								sh.mu.Lock()
								err = sh.stream.WritePacketRTP(sh.media, pkt)
								sh.mu.Unlock()
								if err != nil {
									logger.Debug(err.Error())
									return
								}
							}
						}
					}, sh.wg.Done)
					return &base.Response{StatusCode: base.StatusOK}, nil
				},
				OnRecordFunc: func(ctx *gortsplib.ServerHandlerOnRecordCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnRecordFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
			}
			defer h.wg.Wait()

			h.s = &gortsplib.Server{
				Handler:     h,
				RTSPAddress: "127.0.0.1:32512",
			}
			defer func() {
				test.That(t, h.s.Wait(), test.ShouldBeError, errors.New("terminated"))
			}()
			defer h.s.Close()
			test.That(t, h.s.Start(), test.ShouldBeNil)
			rtspConf := &Config{Address: "rtsp://" + h.s.RTSPAddress}
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer timeoutCancel()
			rtspCam, err := NewRTSPCamera(timeoutCtx, resource.Name{Name: "foo"}, rtspConf, logger)
			test.That(t, err, test.ShouldBeNil)
			im, f, err := camera.ReadImage(timeoutCtx, rtspCam)
			test.That(t, err, test.ShouldBeNil)
			defer f()
			test.That(t, im, test.ShouldNotBeNil)

			test.That(t, im.Bounds(), test.ShouldResemble, image.Rectangle{Max: image.Point{X: 640, Y: 480}})
			Y, Cb, Cr := color.RGBToYCbCr(red.R, red.G, red.B)
			for y := 0; y < im.Bounds().Dy(); y++ {
				for x := 0; x < im.Bounds().Dx(); x++ {
					test.That(t, im.At(x, y), test.ShouldResemble, color.YCbCr{Y: Y, Cb: Cb, Cr: Cr})
				}
			}
			// close everything
			defer func() {
				err = rtspCam.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)
			}()
		})
	})

	t.Run("H264", func(t *testing.T) {
		var forma = &format.H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		}
		t.Run("init", func(t *testing.T) {
			h := &serverHandler{
				media: &description.Media{
					Type:    description.MediaTypeVideo,
					Formats: []format.Format{forma},
				},
				OnConnOpenFunc: func(ctx *gortsplib.ServerHandlerOnConnOpenCtx, sh *serverHandler) {
					logger.Debug("OnConnOpenFunc")
				},
				OnConnCloseFunc: func(ctx *gortsplib.ServerHandlerOnConnCloseCtx, sh *serverHandler) {
					logger.Debug("OnConnCloseFunc")
				},
				OnSessionOpenFunc: func(ctx *gortsplib.ServerHandlerOnSessionOpenCtx, sh *serverHandler) {
					logger.Debug("OnSessionOpenFunc")
				},
				OnSessionCloseFunc: func(ctx *gortsplib.ServerHandlerOnSessionCloseCtx, sh *serverHandler) {
					logger.Debug("OnSessionCloseFunc")
					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream.Close()
				},
				OnDescribeFunc: func(ctx *gortsplib.ServerHandlerOnDescribeCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnDescribeFunc")

					sh.mu.Lock()
					defer sh.mu.Unlock()
					sh.stream = gortsplib.NewServerStream(sh.s, &description.Session{
						BaseURL: bURL,
						Title:   "123456",
						Medias:  []*description.Media{sh.media},
					})
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnAnnounceFunc: func(ctx *gortsplib.ServerHandlerOnAnnounceCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnAnnounceFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
				OnSetupFunc: func(ctx *gortsplib.ServerHandlerOnSetupCtx, sh *serverHandler) (*base.Response, *gortsplib.ServerStream, error) {
					logger.Debug("OnSetupFunc")
					return &base.Response{StatusCode: base.StatusOK}, sh.stream, nil
				},
				OnPlayFunc: func(ctx *gortsplib.ServerHandlerOnPlayCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnPlayFunc")
					return &base.Response{StatusCode: base.StatusOK}, nil
				},
				OnRecordFunc: func(ctx *gortsplib.ServerHandlerOnRecordCtx, sh *serverHandler) (*base.Response, error) {
					logger.Debug("OnRecordFunc")
					t.Log("should not be called")
					t.FailNow()
					return nil, errors.New("should not be called")
				},
			}
			defer h.wg.Wait()

			h.s = &gortsplib.Server{
				Handler:     h,
				RTSPAddress: "127.0.0.1:32512",
			}
			test.That(t, h.s.Start(), test.ShouldBeNil)
			rtspConf := &Config{Address: "rtsp://" + h.s.RTSPAddress, H264Passthrough: true}
			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer timeoutCancel()
			rtspCam, err := NewRTSPCamera(timeoutCtx, resource.Name{Name: "foo"}, rtspConf, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rtspCam.Name().Name, test.ShouldEqual, "foo")
			// close everything
			err = rtspCam.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)
			h.s.Close()
			test.That(t, h.s.Wait(), test.ShouldBeError, errors.New("terminated"))
		})
	})
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

type serverHandler struct {
	s                  *gortsplib.Server
	wg                 sync.WaitGroup
	media              *description.Media
	OnConnOpenFunc     func(*gortsplib.ServerHandlerOnConnOpenCtx, *serverHandler)
	OnConnCloseFunc    func(*gortsplib.ServerHandlerOnConnCloseCtx, *serverHandler)
	OnSessionOpenFunc  func(*gortsplib.ServerHandlerOnSessionOpenCtx, *serverHandler)
	OnSessionCloseFunc func(*gortsplib.ServerHandlerOnSessionCloseCtx, *serverHandler)
	OnDescribeFunc     func(*gortsplib.ServerHandlerOnDescribeCtx, *serverHandler) (*base.Response, *gortsplib.ServerStream, error)
	OnAnnounceFunc     func(*gortsplib.ServerHandlerOnAnnounceCtx, *serverHandler) (*base.Response, error)
	OnSetupFunc        func(*gortsplib.ServerHandlerOnSetupCtx, *serverHandler) (*base.Response, *gortsplib.ServerStream, error)
	OnPlayFunc         func(*gortsplib.ServerHandlerOnPlayCtx, *serverHandler) (*base.Response, error)
	OnRecordFunc       func(*gortsplib.ServerHandlerOnRecordCtx, *serverHandler) (*base.Response, error)
	mu                 sync.Mutex
	stream             *gortsplib.ServerStream
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	sh.OnConnOpenFunc(ctx, sh)
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	sh.OnConnCloseFunc(ctx, sh)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	sh.OnSessionOpenFunc(ctx, sh)
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	sh.OnSessionCloseFunc(ctx, sh)
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	return sh.OnDescribeFunc(ctx, sh)
}

// called when receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	return sh.OnAnnounceFunc(ctx, sh)
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	return sh.OnSetupFunc(ctx, sh)
}

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	return sh.OnPlayFunc(ctx, sh)
}

// called when receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	return sh.OnRecordFunc(ctx, sh)
}
