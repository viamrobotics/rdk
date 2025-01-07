package camera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/viamrobotics/webrtc/v3"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/component/camera/v1"
	streampb "go.viam.com/api/stream/v1"
	goutils "go.viam.com/utils"
	goprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var (
	// ErrNoPeerConnection indicates there was no peer connection.
	ErrNoPeerConnection = errors.New("no PeerConnection")
	// ErrNoSharedPeerConnection indicates there was no shared peer connection.
	ErrNoSharedPeerConnection = errors.New("no Shared PeerConnection")
	// ErrUnknownSubscriptionID indicates that a SubscriptionID is unknown.
	ErrUnknownSubscriptionID = errors.New("subscriptionID Unknown")
	readRTPTimeout           = time.Millisecond * 200
)

type bufAndCB struct {
	cb  func(*rtp.Packet)
	buf *rtppassthrough.Buffer
}

// client implements CameraServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	remoteName              string
	name                    string
	conn                    rpc.ClientConn
	client                  pb.CameraServiceClient
	streamClient            streampb.StreamServiceClient
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup

	healthyClientChMu sync.Mutex
	healthyClientCh   chan struct{}

	rtpPassthroughMu sync.Mutex
	runningStreams   map[rtppassthrough.SubscriptionID]bufAndCB
	subGenerationID  int
	associatedSubs   map[int][]rtppassthrough.SubscriptionID
	trackClosed      <-chan struct{}
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Camera, error) {
	c := pb.NewCameraServiceClient(conn)
	streamClient := streampb.NewStreamServiceClient(conn)
	trackClosed := make(chan struct{})
	close(trackClosed)
	return &client{
		remoteName:     remoteName,
		Named:          name.PrependRemote(remoteName).AsNamed(),
		name:           name.ShortName(),
		conn:           conn,
		streamClient:   streamClient,
		client:         c,
		runningStreams: map[rtppassthrough.SubscriptionID]bufAndCB{},
		trackClosed:    trackClosed,
		associatedSubs: map[int][]rtppassthrough.SubscriptionID{},
		logger:         logger,
	}, nil
}

func (c *client) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Stream")

	// RSDK-6340: The resource manager closes remote resources when the underlying
	// connection goes bad. However, when the connection is re-established, the client
	// objects these resources represent are not re-initialized/marked "healthy".
	// `healthyClientCh` helps track these transitions between healthy and unhealthy
	// states.
	//
	// When a new `client.Stream()` is created we will either use the existing
	// `healthyClientCh` or create a new one.
	//
	// The goroutine a `Stream()` method spins off will listen to its version of the
	// `healthyClientCh` to be notified when the connection has died so it can gracefully
	// terminate.
	//
	// When a connection becomes unhealthy, the resource manager will call `Close` on the
	// camera client object. Closing the client will:
	// 1. close its `client.healthyClientCh` channel
	// 2. wait for existing "stream" goroutines to drain
	// 3. nil out the `client.healthyClientCh` member variable
	//
	// New streams concurrent with closing cannot start until this drain completes. There
	// will never be stream goroutines from the old "generation" running concurrently
	// with those from the new "generation".
	healthyClientCh := c.maybeResetHealthyClientCh()

	mimeTypeFromCtx := gostream.MIMETypeHint(ctx, "")
	ctxWithMIME := gostream.WithMIMETypeHint(context.Background(), mimeTypeFromCtx)
	streamCtx, stream, frameCh := gostream.NewMediaStreamForChannel[image.Image](ctxWithMIME)

	c.activeBackgroundWorkers.Add(1)

	goutils.PanicCapturingGo(func() {
		streamCtx = trace.NewContext(streamCtx, span)
		defer span.End()

		defer c.activeBackgroundWorkers.Done()
		defer close(frameCh)

		for {
			if streamCtx.Err() != nil {
				return
			}

			img, err := DecodeImageFromCamera(streamCtx, mimeTypeFromCtx, nil, c)
			if err != nil {
				for _, handler := range errHandlers {
					handler(streamCtx, err)
				}
			}

			select {
			case <-streamCtx.Done():
				return
			case <-healthyClientCh:
				if err := stream.Close(ctxWithMIME); err != nil {
					c.logger.CWarnw(ctx, "error closing stream", "err", err)
				}
				return
			case frameCh <- gostream.MediaReleasePairWithError[image.Image]{
				Media:   img,
				Release: func() {},
				Err:     err,
			}:
			}
		}
	})

	return stream, nil
}

func (c *client) Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, ImageMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Image")
	defer span.End()
	expectedType, _ := utils.CheckLazyMIMEType(mimeType)

	convertedExtra, err := goprotoutils.StructToStructPb(extra)
	if err != nil {
		return nil, ImageMetadata{}, err
	}
	resp, err := c.client.GetImage(ctx, &pb.GetImageRequest{
		Name:     c.name,
		MimeType: expectedType,
		Extra:    convertedExtra,
	})
	if err != nil {
		return nil, ImageMetadata{}, err
	}
	if len(resp.Image) == 0 {
		return nil, ImageMetadata{}, errors.New("received empty bytes from client GetImage")
	}

	if expectedType != "" && resp.MimeType != expectedType {
		c.logger.CDebugw(ctx, "got different MIME type than what was asked for", "sent", expectedType, "received", resp.MimeType)
		if resp.MimeType == "" {
			// if the user expected a mime_type and the successful response didn't have a mime type, assume the
			// response's mime_type was what the user requested
			resp.MimeType = mimeType
		}
	}

	return resp.Image, ImageMetadata{MimeType: resp.MimeType}, nil
}

func (c *client) Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Images")
	defer span.End()

	resp, err := c.client.GetImages(ctx, &pb.GetImagesRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("camera client: could not gets images from the camera %w", err)
	}

	images := make([]NamedImage, 0, len(resp.Images))
	// keep everything lazy encoded by default, if type is unknown, attempt to decode it
	for _, img := range resp.Images {
		var rdkImage image.Image
		switch img.Format {
		case pb.Format_FORMAT_RAW_RGBA:
			rdkImage = rimage.NewLazyEncodedImage(img.Image, utils.MimeTypeRawRGBA)
		case pb.Format_FORMAT_RAW_DEPTH:
			rdkImage = rimage.NewLazyEncodedImage(img.Image, utils.MimeTypeRawDepth)
		case pb.Format_FORMAT_JPEG:
			rdkImage = rimage.NewLazyEncodedImage(img.Image, utils.MimeTypeJPEG)
		case pb.Format_FORMAT_PNG:
			rdkImage = rimage.NewLazyEncodedImage(img.Image, utils.MimeTypePNG)
		case pb.Format_FORMAT_UNSPECIFIED:
			rdkImage, _, err = image.Decode(bytes.NewReader(img.Image))
			if err != nil {
				return nil, resource.ResponseMetadata{}, err
			}
		}
		images = append(images, NamedImage{rdkImage, img.SourceName})
	}
	return images, resource.ResponseMetadataFromProto(resp.ResponseMetadata), nil
}

func (c *client) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::NextPointCloud")
	defer span.End()

	ctx, getPcdSpan := trace.StartSpan(ctx, "camera::client::NextPointCloud::GetPointCloud")

	extra := make(map[string]interface{})
	if ctx.Value(data.FromDMContextKey{}) == true {
		extra[data.FromDMString] = true
	}
	extraStructPb, err := goprotoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.GetPointCloud(ctx, &pb.GetPointCloudRequest{
		Name:     c.name,
		MimeType: utils.MimeTypePCD,
		Extra:    extraStructPb,
	})
	getPcdSpan.End()
	if err != nil {
		return nil, err
	}

	if resp.MimeType != utils.MimeTypePCD {
		return nil, fmt.Errorf("unknown pc mime type %s", resp.MimeType)
	}

	return func() (pointcloud.PointCloud, error) {
		_, span := trace.StartSpan(ctx, "camera::client::NextPointCloud::ReadPCD")
		defer span.End()

		return pointcloud.ReadPCD(bytes.NewReader(resp.PointCloud))
	}()
}

func (c *client) Properties(ctx context.Context) (Properties, error) {
	result := Properties{}
	resp, err := c.client.GetProperties(ctx, &pb.GetPropertiesRequest{
		Name: c.name,
	})
	if err != nil {
		return Properties{}, err
	}
	if intrinsics := resp.IntrinsicParameters; intrinsics != nil {
		result.IntrinsicParams = &transform.PinholeCameraIntrinsics{
			Width:  int(intrinsics.WidthPx),
			Height: int(intrinsics.HeightPx),
			Fx:     intrinsics.FocalXPx,
			Fy:     intrinsics.FocalYPx,
			Ppx:    intrinsics.CenterXPx,
			Ppy:    intrinsics.CenterYPx,
		}
	}
	result.MimeTypes = resp.MimeTypes
	result.SupportsPCD = resp.SupportsPcd

	// Check if the optional frame_rate is present and set it if it exists
	if resp.FrameRate != nil {
		result.FrameRate = *resp.FrameRate
	}
	// if no distortion model present, return result with no model
	if resp.DistortionParameters == nil {
		return result, nil
	}
	if resp.DistortionParameters.Model == "" { // same as if nil
		return result, nil
	}
	// switch distortion model based on model name
	model := transform.DistortionType(resp.DistortionParameters.Model)
	distorter, err := transform.NewDistorter(model, resp.DistortionParameters.Parameters)
	if err != nil {
		return Properties{}, err
	}
	result.DistortionParams = distorter
	return result, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

// TODO(RSDK-6433): This method can be called more than once during a client's lifecycle.
// For example, consider a case where a remote camera goes offline and then back online.
// We will call `Close` on the camera client when we detect the disconnection to remove
// active streams but then reuse the client when the connection is re-established.
func (c *client) Close(ctx context.Context) error {
	_, span := trace.StartSpan(ctx, "camera::client::Close")
	defer span.End()

	c.healthyClientChMu.Lock()
	if c.healthyClientCh != nil {
		close(c.healthyClientCh)
	}
	c.healthyClientCh = nil
	c.healthyClientChMu.Unlock()

	// unsubscribe from all video streams that have been established with modular cameras
	c.unsubscribeAll()

	// NOTE: (Nick S) we are intentionally releasing the lock before we wait for
	// background goroutines to terminate as some of them need to be able
	// to take the lock to terminate
	c.activeBackgroundWorkers.Wait()
	return nil
}

func (c *client) maybeResetHealthyClientCh() chan struct{} {
	c.healthyClientChMu.Lock()
	defer c.healthyClientChMu.Unlock()
	if c.healthyClientCh == nil {
		c.healthyClientCh = make(chan struct{})
	}
	return c.healthyClientCh
}

// SubscribeRTP begins a subscription to receive RTP packets. SubscribeRTP waits for the VideoTrack
// to be available before returning. The subscription/video is valid until the
// `Subscription.Terminated` context is `Done`.
//
// SubscribeRTP maintains the invariant that there is at most a single track between the client and
// WebRTC peer. Concurrent callers will block and wait for the "winner" to complete.
func (c *client) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::SubscribeRTP")
	defer span.End()
	// RSDK-6340: The resource manager closes remote resources when the underlying
	// connection goes bad. However, when the connection is re-established, the client
	// objects these resources represent are not re-initialized/marked "healthy".
	// `healthyClientCh` helps track these transitions between healthy and unhealthy
	// states.
	//
	// When `client.SubscribeRTP()` is called we will either use the existing
	// `healthyClientCh` or create a new one.
	//
	// The goroutine a `Tracker.AddOnTrackSub()` method spins off will listen to its version of the
	// `healthyClientCh` to be notified when the connection has died so it can gracefully
	// terminate.
	//
	// When a connection becomes unhealthy, the resource manager will call `Close` on the
	// camera client object. Closing the client will:
	// 1. close its `client.healthyClientCh` channel
	// 2. wait for existing "SubscribeRTP" goroutines to drain
	// 3. nil out the `client.healthyClientCh` member variable
	//
	// New streams concurrent with closing cannot start until this drain completes. There
	// will never be stream goroutines from the old "generation" running concurrently
	// with those from the new "generation".
	healthyClientCh := c.maybeResetHealthyClientCh()

	c.rtpPassthroughMu.Lock()
	defer c.rtpPassthroughMu.Unlock()

	// Create a Subscription object and allocate a ring buffer of RTP packets.
	sub, rtpPacketBuffer, err := rtppassthrough.NewSubscription(bufferSize)
	if err != nil {
		return sub, err
	}
	g := utils.NewGuard(func() {
		c.logger.CDebug(ctx, "Error subscribing to RTP. Closing passthrough buffer.")
		rtpPacketBuffer.Close()
	})
	defer g.OnFail()

	// RTP Passthrough only works over PeerConnection video tracks.
	if c.conn.PeerConn() == nil {
		return rtppassthrough.NilSubscription, ErrNoPeerConnection
	}

	// Calling `AddStream(streamId)` on the remote viam-server/module will eventually result in
	// invoking a local `PeerConnection.OnTrack(streamId, rtpReceiver)` callback. In this callback
	// we set up the goroutine to read video data from the remote side and copy those packets
	// upstream.
	//
	// webrtc.PeerConnection objects can only install a single `OnTrack` callback. Because there
	// could be multiple video stream sources on the other side of a PeerConnection, we must play a
	// game where we only create one `OnTrack` callback, but that callback object does a map lookup
	// for which video stream. The `grpc.Tracker` API is for manipulating that map.
	tracker, ok := c.conn.(grpc.Tracker)
	if !ok {
		c.logger.CErrorw(ctx, "Client conn is not a `Tracker`", "connType", fmt.Sprintf("%T", c.conn))
		return rtppassthrough.NilSubscription, ErrNoSharedPeerConnection
	}

	c.logger.CDebugw(ctx, "SubscribeRTP", "subID", sub.ID.String(), "name", c.Name(), "bufAndCBByID", c.bufAndCBToString())
	defer func() {
		c.logger.CDebugw(ctx, "SubscribeRTP after", "subID", sub.ID.String(),
			"name", c.Name(), "bufAndCBByID", c.bufAndCBToString())
	}()

	// If we do not currently have a video stream open for this camera, we create one. Otherwise
	// we'll piggy back on the existing stream. We piggy-back for two reasons:
	// 1. Not doing so would result in two webrtc tracks for the same camera sending the exact same
	//    RTP packets. This would needlessly waste resources.
	// 2. The signature of `RemoveStream` just takes the camera name. If we had two streams open for
	//    the same camera when the remote receives `RemoveStream`, it wouldn't know which to stop
	//    sending data for.
	if len(c.runningStreams) == 0 {
		c.logger.CInfow(ctx, "SubscribeRTP is creating a video track",
			"client", fmt.Sprintf("%p", c), "peerConnection", fmt.Sprintf("%p", c.conn.PeerConn()))
		// A previous subscriber/track may have exited, but its resources have not necessarily been
		// cleaned up. We must wait for that to complete. We additionally select on other error
		// conditions.
		select {
		case <-c.trackClosed:
		case <-ctx.Done():
			return rtppassthrough.NilSubscription, fmt.Errorf("track not closed within SubscribeRTP provided context %w", ctx.Err())
		case <-healthyClientCh:
			return rtppassthrough.NilSubscription, errors.New("camera client is closed")
		}

		// Create channels for two important events: when a video track is established an when a
		// video track exits. I.e: When the `OnTrack` callback is invoked and when the `RTPReceiver`
		// is no longer being read from.
		trackReceived, trackClosed := make(chan struct{}), make(chan struct{})

		// We're creating a new video track. Bump the generation ID and associate all new
		// subscriptions to this generation.
		c.subGenerationID++
		c.associatedSubs[c.subGenerationID] = []rtppassthrough.SubscriptionID{}

		// Add the camera's addOnTrackSubFunc to the SharedConn's map of OnTrack callbacks.
		tracker.AddOnTrackSub(c.trackName(), c.addOnTrackFunc(healthyClientCh, trackReceived, trackClosed, c.subGenerationID))
		// Remove the OnTrackSub once we either fail or succeed.
		defer tracker.RemoveOnTrackSub(c.trackName())

		// Call `AddStream` on the remote. In the successful case, this will result in a
		// PeerConnection renegotiation to add this camera's video track and have the `OnTrack`
		// callback invoked.
		if _, err := c.streamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: c.trackName()}); err != nil {
			c.logger.CDebugw(ctx, "SubscribeRTP AddStream hit error", "subID", sub.ID.String(), "trackName", c.trackName(), "err", err)
			return rtppassthrough.NilSubscription, err
		}

		c.logger.CDebugw(ctx, "SubscribeRTP waiting for track", "client", fmt.Sprintf("%p", c), "pc", fmt.Sprintf("%p", c.conn.PeerConn()))
		select {
		case <-ctx.Done():
			return rtppassthrough.NilSubscription, fmt.Errorf("track not received within SubscribeRTP provided context %w", ctx.Err())
		case <-healthyClientCh:
			return rtppassthrough.NilSubscription, errors.New("track not received before client closed")
		case <-trackReceived:
			c.logger.CDebugw(ctx, "SubscribeRTP received track data", "client", fmt.Sprintf("%p", c), "pc", fmt.Sprintf("%p", c.conn.PeerConn()))
		}

		// Set up channel to detect when the track has closed. This can happen in response to an
		// event / error internal to the peer or due to calling `RemoveStream`.
		c.trackClosed = trackClosed
		c.logger.CDebugw(ctx, "SubscribeRTP called AddStream and succeeded", "subID", sub.ID.String())
	}

	// Associate this subscription with the current generation.
	c.associatedSubs[c.subGenerationID] = append(c.associatedSubs[c.subGenerationID], sub.ID)

	// Add the subscription to runningStreams. The goroutine spawned by `addOnTrackFunc` will use
	// this to know where to forward incoming RTP packets.
	c.runningStreams[sub.ID] = bufAndCB{
		cb:  func(p *rtp.Packet) { packetsCB([]*rtp.Packet{p}) },
		buf: rtpPacketBuffer,
	}

	// Start the goroutine that copies RTP packets
	rtpPacketBuffer.Start()
	g.Success()
	c.logger.CDebugw(ctx, "SubscribeRTP succeeded", "subID", sub.ID.String(),
		"name", c.Name(), "bufAndCBByID", c.bufAndCBToString())
	return sub, nil
}

func (c *client) addOnTrackFunc(
	healthyClientCh, trackReceived, trackClosed chan struct{},
	generationID int,
) grpc.OnTrackCB {
	// This is invoked when `PeerConnection.OnTrack` is called for this camera's stream id.
	return func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		// Our `OnTrack` was called. Inform `SubscribeRTP` that getting video data was successful.
		close(trackReceived)
		c.activeBackgroundWorkers.Add(1)
		goutils.ManagedGo(func() {
			var count atomic.Uint64
			defer close(trackClosed)
			for {
				select {
				case <-healthyClientCh:
					c.logger.Debugw("OnTrack callback terminating as the client is closing", "parentID", generationID)
					return
				default:
				}

				// We set a read deadline such that we don't block in I/O. This allows us to respond
				// to events such as shutdown. Normally this would be done with a context on the
				// `ReadRTP` method.
				deadline := time.Now().Add(readRTPTimeout)
				if err := tr.SetReadDeadline(deadline); err != nil {
					c.logger.Errorw("SetReadDeadline error", "generationId", generationID, "err", err)
					return
				}

				pkt, _, err := tr.ReadRTP()
				if os.IsTimeout(err) {
					c.logger.Debugw("ReadRTP read timeout", "generationId", generationID,
						"err", err, "timeout", readRTPTimeout.String())
					continue
				}

				if err != nil {
					if errors.Is(err, io.EOF) {
						c.logger.Infow("ReadRTP received EOF", "generationId", generationID, "err", err)
					} else {
						c.logger.Warnw("ReadRTP error", "generationId", generationID, "err", err)
					}
					// NOTE: (Nick S) We need to remember which subscriptions are consuming packets
					// from to which tr *webrtc.TrackRemote so that we can terminate the child subscriptions
					// when their track terminate.
					c.unsubscribeChildrenSubs(generationID)
					return
				}

				c.rtpPassthroughMu.Lock()
				for _, tmp := range c.runningStreams {
					if count.Add(1)%10000 == 0 {
						c.logger.Debugw("ReadRTP called. Sampling 1/10000", "count", count.Load(), "packetTs", pkt.Header.Timestamp)
					}

					// This is needed to prevent the problem described here:
					// https://go.dev/blog/loopvar-preview
					bufAndCB := tmp
					err := bufAndCB.buf.Publish(func() { bufAndCB.cb(pkt) })
					if err != nil {
						c.logger.Debugw("rtp passthrough packet dropped",
							"generationId", generationID, "err", err)
					}
				}
				c.rtpPassthroughMu.Unlock()
			}
		}, c.activeBackgroundWorkers.Done)
	}
}

// Unsubscribe terminates a subscription to receive RTP packets.
//
// It is strongly recommended to set a timeout on ctx as the underlying WebRTC peer connection's
// Track can be removed before sending an RTP packet which results in Unsubscribe blocking until the
// provided context is cancelled or the client is closed.  This rare condition may happen in cases
// like reconfiguring concurrently with an Unsubscribe call.
func (c *client) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	ctx, span := trace.StartSpan(ctx, "camera::client::Unsubscribe")
	defer span.End()

	if c.conn.PeerConn() == nil {
		return ErrNoPeerConnection
	}

	c.healthyClientChMu.Lock()
	healthyClientCh := c.healthyClientCh
	c.healthyClientChMu.Unlock()

	c.logger.CInfow(ctx, "Unsubscribe called", "subID", id.String())
	c.rtpPassthroughMu.Lock()
	bufAndCB, ok := c.runningStreams[id]
	if !ok {
		c.logger.CWarnw(ctx, "Unsubscribe called with unknown subID", "subID", id.String())
		c.rtpPassthroughMu.Unlock()
		return ErrUnknownSubscriptionID
	}

	if len(c.runningStreams) > 1 {
		// There are still existing output streams for this camera. We close the resources
		// associated with the input `SubscriptionID` and return.
		delete(c.runningStreams, id)
		c.rtpPassthroughMu.Unlock()
		bufAndCB.buf.Close()
		return nil
	}

	// There is only one stream left. In addition to closing the input subscription's resources, we
	// also close the stream from the underlying remote.
	//
	// Calling `RemoveStream` releases resources on the remote. We promise to keep retrying until
	// we're successful.
	c.logger.CInfow(ctx, "Last subscriber. calling RemoveStream",
		"trackName", c.trackName(), "subID", id.String())
	request := &streampb.RemoveStreamRequest{Name: c.trackName()}
	// We assume the server responds with a success if the requested `Name` is unknown/already
	// removed.
	if _, err := c.streamClient.RemoveStream(ctx, request); err != nil {
		c.logger.CWarnw(ctx, "Unsubscribe RemoveStream returned err", "trackName",
			c.trackName(), "subID", id.String(), "err", err)
		c.rtpPassthroughMu.Unlock()
		return err
	}

	// We can only remove `runningStreams` when `RemoveStream` succeeds. If there was an error, the
	// upper layers are responsible for retrying `Unsubscribe`. At this point we will not return an
	// error to clearly inform the caller that `Unsubscribe` does not need to be retried.
	delete(c.runningStreams, id)
	bufAndCB.buf.Close()

	// The goroutine we're stopping uses the `rtpPassthroughMu` to access the map of
	// `runningStreams`. We must unlock while we wait for it to exit.
	c.rtpPassthroughMu.Unlock()

	select {
	case <-c.trackClosed:
		return nil
	case <-ctx.Done():
		c.logger.CWarnw(ctx, "RemoveStream successfully called, but errored waiting for stream to send an EOF",
			"subID", id.String(), "err", ctx.Err())
		return nil
	case <-healthyClientCh:
		c.logger.CWarnw(ctx, "RemoveStream successfully called, but camera closed waiting for stream to send an EOF",
			"subID", id.String())
		return nil
	}
}

func (c *client) trackName() string {
	// if c.conn is a *grpc.SharedConn then the client
	// is talking to a module and we need to send the fully qualified name
	if _, ok := c.conn.(*grpc.SharedConn); ok {
		return c.Name().String()
	}

	// otherwise we know we are talking to either a main or remote robot part
	if c.remoteName != "" {
		// if c.remoteName != "" it indicates that we are talking to a remote part & we need to pop the remote name
		// as the remote doesn't know it's own name from the perspective of the main part
		return c.Name().PopRemote().SDPTrackName()
	}
	// in this case we are talking to a main part & the remote name (if it exists) needs to be preserved
	return c.Name().SDPTrackName()
}

func (c *client) unsubscribeAll() {
	c.rtpPassthroughMu.Lock()
	defer c.rtpPassthroughMu.Unlock()
	if len(c.runningStreams) > 0 {
		// shutdown & delete all *rtppassthrough.Buffer and callbacks
		for subID, bufAndCB := range c.runningStreams {
			c.logger.Debugw("unsubscribeAll terminating sub", "subID", subID.String())
			delete(c.runningStreams, subID)
			bufAndCB.buf.Close()
		}
	}
}

func (c *client) unsubscribeChildrenSubs(generationID int) {
	c.rtpPassthroughMu.Lock()
	defer c.rtpPassthroughMu.Unlock()
	c.logger.Debugw("client unsubscribeChildrenSubs called", "generationId", generationID, "numSubs", len(c.associatedSubs))
	defer c.logger.Debugw("client unsubscribeChildrenSubs done", "generationId", generationID)
	for _, subID := range c.associatedSubs[generationID] {
		bufAndCB, ok := c.runningStreams[subID]
		if !ok {
			continue
		}
		c.logger.Debugw("stopping subscription", "generationId", generationID, "subId", subID.String())
		delete(c.runningStreams, subID)
		bufAndCB.buf.Close()
	}
	delete(c.associatedSubs, generationID)
}

func (c *client) bufAndCBToString() string {
	if len(c.runningStreams) == 0 {
		return "len: 0"
	}
	strIDs := []string{}
	strIDsToCB := map[string]bufAndCB{}
	for id, cb := range c.runningStreams {
		strID := id.String()
		strIDs = append(strIDs, strID)
		strIDsToCB[strID] = cb
	}
	slices.Sort(strIDs)
	ret := fmt.Sprintf("len: %d, ", len(c.runningStreams))
	for _, strID := range strIDs {
		ret += fmt.Sprintf("%s: %v, ", strID, strIDsToCB[strID])
	}
	return ret
}
