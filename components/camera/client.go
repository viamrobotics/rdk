package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"slices"
	"sync"

	"github.com/pion/rtp"
	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/component/camera/v1"
	streampb "go.viam.com/api/stream/v1"
	goutils "go.viam.com/utils"
	goprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	rdkgrpc "go.viam.com/rdk/grpc"
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
	ErrNoPeerConnection = errors.New("No PeerConnection")
	// ErrNoSharedPeerConnection indicates there was no shared peer connection.
	ErrNoSharedPeerConnection = errors.New("No Shared PeerConnection")
	// ErrUnknownSubscriptionID indicates that a SubscriptionID is unknown.
	ErrUnknownSubscriptionID = errors.New("SubscriptionID Unknown")
)

type (
	singlePacketCallback func(*rtp.Packet)
	bufAndCB             struct {
		cb  singlePacketCallback
		buf *rtppassthrough.Buffer
	}
	bufAndCBByID map[rtppassthrough.SubscriptionID]bufAndCB
)

// client implements CameraServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	ctx                     context.Context
	cancelFn                context.CancelFunc
	name                    string
	conn                    rpc.ClientConn
	client                  pb.CameraServiceClient
	streamClient            streampb.StreamServiceClient
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup

	mu                  sync.Mutex
	healthyClientCh     chan struct{}
	bufAndCBByID        bufAndCBByID
	currentSubParentID  rtppassthrough.SubscriptionID
	subParentToChildren map[rtppassthrough.SubscriptionID][]rtppassthrough.SubscriptionID
	trackClosed         <-chan struct{}
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
	closeCtx, cancelFn := context.WithCancel(context.Background())
	return &client{
		ctx:                 closeCtx,
		cancelFn:            cancelFn,
		Named:               name.PrependRemote(remoteName).AsNamed(),
		name:                name.ShortName(),
		conn:                conn,
		streamClient:        streamClient,
		client:              c,
		bufAndCBByID:        map[rtppassthrough.SubscriptionID]bufAndCB{},
		trackClosed:         trackClosed,
		subParentToChildren: map[rtppassthrough.SubscriptionID][]rtppassthrough.SubscriptionID{},
		logger:              logger,
	}, nil
}

func getExtra(ctx context.Context) (*structpb.Struct, error) {
	ext := &structpb.Struct{}
	if extra, ok := FromContext(ctx); ok {
		var err error
		if ext, err = goprotoutils.StructToStructPb(extra); err != nil {
			return nil, err
		}
	}

	dataExt, err := data.GetExtraFromContext(ctx)
	if err != nil {
		return nil, err
	}

	proto.Merge(ext, dataExt)
	return ext, nil
}

func (c *client) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Read")
	defer span.End()
	mimeType := gostream.MIMETypeHint(ctx, "")
	expectedType, _ := utils.CheckLazyMIMEType(mimeType)

	ext, err := getExtra(ctx)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.client.GetImage(ctx, &pb.GetImageRequest{
		Name:     c.name,
		MimeType: expectedType,
		Extra:    ext,
	})
	if err != nil {
		return nil, nil, err
	}

	if expectedType != "" && resp.MimeType != expectedType {
		c.logger.CDebugw(ctx, "got different MIME type than what was asked for", "sent", expectedType, "received", resp.MimeType)
	} else {
		resp.MimeType = mimeType
	}

	resp.MimeType = utils.WithLazyMIMEType(resp.MimeType)
	img, err := rimage.DecodeImage(ctx, resp.Image, resp.MimeType)
	if err != nil {
		return nil, nil, err
	}
	return img, func() {}, nil
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
	c.mu.Lock()
	if c.healthyClientCh == nil {
		c.healthyClientCh = make(chan struct{})
	}
	healthyClientCh := c.healthyClientCh
	c.mu.Unlock()

	ctxWithMIME := gostream.WithMIMETypeHint(context.Background(), gostream.MIMETypeHint(ctx, ""))
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

			frame, release, err := c.Read(streamCtx)
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
				Media:   frame,
				Release: release,
				Err:     err,
			}:
			}
		}
	})

	return stream, nil
}

func (c *client) Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::Images")
	defer span.End()

	resp, err := c.client.GetImages(ctx, &pb.GetImagesRequest{
		Name: c.name,
	})
	if err != nil {
		return nil, resource.ResponseMetadata{}, errors.Wrap(err, "camera client: could not gets images from the camera")
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

	ext, err := data.GetExtraFromContext(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.GetPointCloud(ctx, &pb.GetPointCloudRequest{
		Name:     c.name,
		MimeType: utils.MimeTypePCD,
		Extra:    ext,
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

	c.cancelFn()
	c.mu.Lock()

	if c.healthyClientCh != nil {
		close(c.healthyClientCh)
	}
	c.healthyClientCh = nil
	// unsubscribe from all video streams that have been established with modular cameras
	c.unsubscribeAll()

	// NOTE: (Nick S) we are intentionally releasing the lock before we wait for
	// background goroutines to terminate as some of them need to be able
	// to take the lock to terminate
	c.mu.Unlock()
	c.activeBackgroundWorkers.Wait()
	return nil
}

// SubscribeRTP begins a subscription to receive RTP packets.
// When the Subscription terminates the context in the returned Subscription
// is cancelled.
// It maintains the invariant that there is at most a single track between
// the client and WebRTC peer.
// It is strongly recommended to set a timeout on ctx as the underlying
// WebRTC peer connection's Track can be removed before sending an RTP packet which
// results in SubscribeRTP blocking until the provided context is cancelled or the client
// is closed.
// This rare condition may happen in cases like reconfiguring concurrently with
// a SubscribeRTP call.
func (c *client) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::SubscribeRTP")
	defer span.End()
	c.mu.Lock()
	defer c.mu.Unlock()
	sub, buf, err := rtppassthrough.NewSubscription(bufferSize)
	if err != nil {
		return sub, err
	}
	g := utils.NewGuard(func() {
		buf.Close()
	})
	defer g.OnFail()

	if c.conn.PeerConn() == nil {
		return rtppassthrough.NilSubscription, ErrNoPeerConnection
	}

	// check if we have established a connection that can be shared by multiple clients asking for cameras streams from viam server.
	sc, ok := c.conn.(*rdkgrpc.SharedConn)
	if !ok {
		return rtppassthrough.NilSubscription, ErrNoSharedPeerConnection
	}

	c.logger.CDebugw(ctx, "SubscribeRTP", "subID", sub.ID.String(), "name", c.Name(), "bufAndCBByID", c.bufAndCBByID.String())
	defer func() {
		c.logger.CDebugw(ctx, "SubscribeRTP after", "subID", sub.ID.String(),
			"name", c.Name(), "bufAndCBByID", c.bufAndCBByID.String())
	}()
	// B/c there is only ever either 0 or 1 peer connections between a module & a viam-server
	// once AddStream is called on the module for a given camera model instance & succeeds, we shouldn't
	// call it again until the previous track is terminated (by calling RemoveStream) for a few reasons:
	// 1. doing so would result in 2 webrtc tracks for the same camera sending the exact same RTP packets which would
	// needlessly waste resources
	// 2. b/c the signature of RemoveStream just takes the camera name, if there are 2 streams for the same camera
	// & the module receives a call to RemoveStream, there is no way for the module to know which camera stream
	// should be removed
	if len(c.bufAndCBByID) == 0 {
		// Wait for previous track to terminate or for the client to close
		select {
		case <-ctx.Done():
			err := errors.Wrap(ctx.Err(), "Track not closed within SubscribeRTP provided context")
			c.logger.Error(err)
			return rtppassthrough.NilSubscription, err
		case <-c.ctx.Done():
			err := errors.Wrap(c.ctx.Err(), "Track not closed before client closed")
			c.logger.Error(err)
			return rtppassthrough.NilSubscription, err
		case <-c.trackClosed:
		}
		trackReceived, trackClosed := make(chan struct{}), make(chan struct{})
		// add the camera model's addOnTrackSubFunc to the shared peer connection's
		// slice of OnTrack callbacks. This is what allows
		// all the bufAndCBByID's callback functions to be called with the
		// RTP packets from the module's peer connection's track
		sc.AddOnTrackSub(c.Name(), c.addOnTrackSubFunc(trackReceived, trackClosed, sub.ID))
		// remove the OnTrackSub once we either fail or succeed
		defer sc.RemoveOnTrackSub(c.Name())
		if _, err := c.streamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: c.Name().String()}); err != nil {
			c.logger.CDebugw(ctx, "SubscribeRTP AddStream hit error", "subID", sub.ID.String(), "name", c.Name(), "err", err)
			return rtppassthrough.NilSubscription, err
		}
		// NOTE: (Nick S) This is a workaround to a Pion bug / missing feature.

		// If the WebRTC peer on the other side of the PeerConnection calls pc.AddTrack followd by pc.RemoveTrack
		// before the module writes RTP packets
		// to the track, the client's PeerConnection.OnTrack callback is never called.
		// That results in the client never receiving RTP packets on subscriptions which never terminate
		// which is an unacceptable failure mode.

		// To prevent that failure mode, we exit with an error if a track is not received within
		// the SubscribeRTP context.
		select {
		case <-ctx.Done():
			err := errors.Wrap(ctx.Err(), "Track not received within SubscribeRTP provided context")
			c.logger.Error(err.Error())
			return rtppassthrough.NilSubscription, err
		case <-c.ctx.Done():
			err := errors.Wrap(c.ctx.Err(), "Track not received before client closed")
			c.logger.Error(err)
			return rtppassthrough.NilSubscription, err
		case <-trackReceived:
			c.logger.Debug("received track")
		}
		// set up channel so we can detect when the track has closed (in response to an event / error internal to the
		// peer or due to calling RemoveStream)
		c.trackClosed = trackClosed
		// the sub is the new parent of all subsequent subs until the number if subsriptions falls back to 0
		c.currentSubParentID = sub.ID
		c.subParentToChildren[c.currentSubParentID] = []rtppassthrough.SubscriptionID{}
		c.logger.CDebugw(ctx, "SubscribeRTP called AddStream and succeeded", "subID", sub.ID.String(),
			"name", c.Name())
	}
	c.subParentToChildren[c.currentSubParentID] = append(c.subParentToChildren[c.currentSubParentID], sub.ID)
	// add the subscription to bufAndCBByID so the goroutine spawned by
	// addOnTrackSubFunc can forward the packets it receives from the modular camera
	// over WebRTC to the SubscribeRTP caller via the packetsCB callback
	c.bufAndCBByID[sub.ID] = bufAndCB{
		cb:  func(p *rtp.Packet) { packetsCB([]*rtp.Packet{p}) },
		buf: buf,
	}
	buf.Start()
	g.Success()
	c.logger.CDebugw(ctx, "SubscribeRTP succeeded", "subID", sub.ID.String(),
		"name", c.Name(), "bufAndCBByID", c.bufAndCBByID.String())
	return sub, nil
}

func (c *client) addOnTrackSubFunc(
	trackReceived, trackClosed chan struct{},
	parentID rtppassthrough.SubscriptionID,
) rdkgrpc.OnTrackCB {
	return func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		close(trackReceived)
		c.activeBackgroundWorkers.Add(1)
		goutils.ManagedGo(func() {
			for {
				if c.ctx.Err() != nil {
					c.logger.Debugw("SubscribeRTP: camera client", "name ", c.Name(), "parentID", parentID.String(),
						"OnTrack callback terminating as the client is closing")
					close(trackClosed)
					return
				}

				pkt, _, err := tr.ReadRTP()
				if err != nil {
					close(trackClosed)
					// NOTE: (Nick S) We need to remember which subscriptions are consuming packets
					// from to which tr *webrtc.TrackRemote so that we can terminate the child subscriptions
					// when their track terminate.
					c.unsubscribeChildrenSubs(parentID)
					if errors.Is(err, io.EOF) {
						c.logger.Debugw("SubscribeRTP: camera client", "name ", c.Name(), "parentID", parentID.String(),
							"OnTrack callback terminating ReadRTP loop due to ", err.Error())
						return
					}
					c.logger.Errorw("SubscribeRTP: camera client", "name ", c.Name(), "parentID", parentID.String(),
						"OnTrack callback hit unexpected error from ReadRTP err:", err.Error())
					return
				}

				c.mu.Lock()
				for _, tmp := range c.bufAndCBByID {
					// This is needed to prevent the problem described here:
					// https://go.dev/blog/loopvar-preview
					bufAndCB := tmp
					err := bufAndCB.buf.Publish(func() { bufAndCB.cb(pkt) })
					if err != nil {
						c.logger.Debugw("SubscribeRTP: camera client",
							"name", c.Name(),
							"parentID", parentID.String(),
							"dropped an RTP packet dropped due to",
							"err", err.Error())
					}
				}
				c.mu.Unlock()
			}
		}, c.activeBackgroundWorkers.Done)
	}
}

// Unsubscribe terminates a subscription to receive RTP packets.
// It is strongly recommended to set a timeout on ctx as the underlying
// WebRTC peer connection's Track can be removed before sending an RTP packet which
// results in Unsubscribe blocking until the provided
// context is cancelled or the client is closed.
// This rare condition may happen in cases like reconfiguring concurrently with
// an Unsubscribe call.
func (c *client) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	ctx, span := trace.StartSpan(ctx, "camera::client::Unsubscribe")
	defer span.End()
	c.mu.Lock()

	if c.conn.PeerConn() == nil {
		c.mu.Unlock()
		return ErrNoPeerConnection
	}

	_, ok := c.conn.(*rdkgrpc.SharedConn)
	if !ok {
		c.mu.Unlock()
		return ErrNoSharedPeerConnection
	}
	c.logger.CDebugw(ctx, "Unsubscribe called with", "name", c.Name(), "subID", id.String())

	bufAndCB, ok := c.bufAndCBByID[id]
	if !ok {
		c.logger.CWarnw(ctx, "Unsubscribe called with unknown subID ", "name", c.Name(), "subID", id.String())
		c.mu.Unlock()
		return ErrUnknownSubscriptionID
	}

	if len(c.bufAndCBByID) == 1 {
		c.logger.CDebugw(ctx, "Unsubscribe calling RemoveStream", "name", c.Name(), "subID", id.String())
		if _, err := c.streamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{Name: c.Name().String()}); err != nil {
			c.logger.CWarnw(ctx, "Unsubscribe RemoveStream returned err", "name", c.Name(), "subID", id.String(), "err", err)
			c.mu.Unlock()
			return err
		}

		delete(c.bufAndCBByID, id)
		bufAndCB.buf.Close()

		// unlock so that the OnTrack callback can get the lock if it needs to before the ReadRTP method returns an error
		// which will close `c.trackClosed`.
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			err := errors.Wrap(ctx.Err(), "track not closed within Unsubscribe provided context. Subscription may be left in broken state")
			c.logger.Error(err)
			return err
		case <-c.ctx.Done():
			err := errors.Wrap(c.ctx.Err(), "track not closed before client closed")
			c.logger.Error(err)
			return err
		case <-c.trackClosed:
		}
		return nil
	}
	c.mu.Unlock()

	delete(c.bufAndCBByID, id)
	bufAndCB.buf.Close()

	return nil
}

func (c *client) unsubscribeAll() {
	if len(c.bufAndCBByID) > 0 {
		for id, bufAndCB := range c.bufAndCBByID {
			c.logger.Debugw("unsubscribeAll terminating sub", "name", c.Name(), "subID", id.String())
			delete(c.bufAndCBByID, id)
			bufAndCB.buf.Close()
		}
	}
}

func (c *client) unsubscribeChildrenSubs(parentID rtppassthrough.SubscriptionID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger.Debugw("client unsubscribeChildrenSubs called", "name", c.Name(), "parentID", parentID.String())
	defer c.logger.Debugw("client unsubscribeChildrenSubs done", "name", c.Name(), "parentID", parentID.String())
	for _, childID := range c.subParentToChildren[parentID] {
		bufAndCB, ok := c.bufAndCBByID[childID]
		if !ok {
			continue
		}
		c.logger.Debugw("stopping child subscription", "name", c.Name(), "parentID", parentID.String(), "childID", childID.String())
		delete(c.bufAndCBByID, childID)
		bufAndCB.buf.Close()
	}
	delete(c.subParentToChildren, parentID)
}

func (s bufAndCBByID) String() string {
	if len(s) == 0 {
		return "len: 0"
	}
	strIds := []string{}
	strIdsToCB := map[string]bufAndCB{}
	for id, cb := range s {
		strID := id.String()
		strIds = append(strIds, strID)
		strIdsToCB[strID] = cb
	}
	slices.Sort(strIds)
	ret := fmt.Sprintf("len: %d, ", len(s))
	for _, strID := range strIds {
		ret += fmt.Sprintf("%s: %v, ", strID, strIdsToCB[strID])
	}
	return ret
}
