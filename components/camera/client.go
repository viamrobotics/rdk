package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
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
	// ErrUnknownStreamSubscriptionID indicates that a StreamSubscriptionID is unknown.
	ErrUnknownStreamSubscriptionID    = errors.New("StreamSubscriptionID Unknown")
	unsubscribeAllRemoveStreamTimeout = time.Second
)

type (
	singlePacketCallback func(*rtp.Packet) error
	subAndCallback       struct {
		cb  singlePacketCallback
		sub *rtppassthrough.StreamSubscription
	}
	subAndCallbackByID map[rtppassthrough.SubscriptionID]subAndCallback
)

// client implements CameraServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	name                    string
	conn                    rpc.ClientConn
	client                  pb.CameraServiceClient
	streamClient            streampb.StreamServiceClient
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup

	mu                 sync.Mutex
	healthyClientCh    chan struct{}
	subAndCallbackByID subAndCallbackByID
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
	return &client{
		Named:              name.PrependRemote(remoteName).AsNamed(),
		name:               name.ShortName(),
		conn:               conn,
		streamClient:       streamClient,
		client:             c,
		subAndCallbackByID: map[rtppassthrough.SubscriptionID]subAndCallback{},
		logger:             logger,
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

	if resp.MimeType != expectedType {
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

func (c *client) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	props, err := c.Properties(ctx)
	if err != nil {
		return nil, err
	}
	intrinsics := props.IntrinsicParams
	err = intrinsics.CheckValid()
	if err != nil {
		return nil, err
	}
	proj = intrinsics
	return proj, nil
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

	c.mu.Lock()
	// NOTE: we are intentionally releasing the lock before we wait for
	// background goroutines to terminate as some of them need to be able
	// to take the lock to terminate
	defer c.activeBackgroundWorkers.Wait()
	defer c.mu.Unlock()

	if c.healthyClientCh != nil {
		close(c.healthyClientCh)
	}
	c.healthyClientCh = nil

	// unsubscribe from all video streams that have been established with modular cameras
	if err := c.unsubscribeAll(); err != nil {
		c.logger.CErrorw(ctx, "Close > unsubscribeAll", "err", err, "name", c.Name())
		return err
	}

	return nil
}

func (c *client) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.SubscriptionID, error) {
	ctx, span := trace.StartSpan(ctx, "camera::client::SubscribeRTP")
	defer span.End()
	c.mu.Lock()
	defer c.mu.Unlock()

	onError := func(err error) { c.logger.CErrorw(ctx, "StreamSubscription returned hit an error", "err", err) }
	sub, err := rtppassthrough.NewStreamSubscription(bufferSize, onError)
	if err != nil {
		return uuid.Nil, err
	}

	if c.conn.PeerConn() == nil {
		return uuid.Nil, ErrNoPeerConnection
	}

	// check if we have established a connection that can be shared by multiple clients asking for cameras streams from viam server.
	sc, ok := c.conn.(*rdkgrpc.SharedConn)
	if !ok {
		return uuid.Nil, ErrNoSharedPeerConnection
	}
	c.logger.CDebugw(ctx, "SubscribeRTP", "subID", sub.ID().String(), "name", c.Name(), "subAndCallbackByID", c.subAndCallbackByID.String())
	defer func() {
		c.logger.CDebugw(ctx, "SubscribeRTP after", "subID", sub.ID().String(),
			"name", c.Name(), "subAndCallbackByID", c.subAndCallbackByID.String())
	}()

	// add the subscription to subAndCallbackByID so the goroutine spawned by
	// addOnTrackSubFunc can forward the packets it receives from the modular camera
	// over WebRTC to the SubscribeRTP caller via the packetsCB callback
	c.subAndCallbackByID[sub.ID()] = subAndCallback{
		cb:  func(p *rtp.Packet) error { return packetsCB([]*rtp.Packet{p}) },
		sub: sub,
	}

	// Start the subscription to process calls to sub.Publish
	sub.Start()
	// add the camera model's addOnTrackSubFunc to the shared peer connection's
	// slice of OnTrack callbacks. This is what allows
	// all the subAndCallbackByID's callback functions to be called with the
	// RTP packets from the module's peer connection's track
	sc.AddOnTrackSub(c.Name(), c.addOnTrackSubFunc)

	// B/c there is only ever either 0 or 1 peer connections between a module & a viam-server
	// once AddStream is called on the module for a given camera model instance & succeeds, we shouldn't
	// call it again until RemoveStream is called for a few reasons:
	// 1. doing so would result in 2 webrtc tracks for the same camera sending the exact same RTP packets which would
	// needlessly waste resources
	// 2. b/c the signature of RemoveStream just takes the camera name, if there are 2 streams for the same camera
	// & the module receives a call to RemoveStream, there is no way for the module to know which camera stream
	// should be removed
	if len(c.subAndCallbackByID) == 1 {
		if _, err := c.streamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: c.Name().String()}); err != nil {
			c.logger.CDebugw(ctx, "SubscribeRTP AddStream hit error", "subID", sub.ID().String(), "name", c.Name(), "err", err)
			sub.Close()
			delete(c.subAndCallbackByID, sub.ID())
			sc.RemoveOnTrackSub(c.Name())
			return uuid.Nil, err
		}
	}
	c.logger.CInfow(ctx, "SubscribeRTP succeeded", "subID", sub.ID().String(),
		"name", c.Name(), "subAndCallbackByID", c.subAndCallbackByID.String())
	return sub.ID(), nil
}

func (c *client) addOnTrackSubFunc(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Name().String() != tr.StreamID() {
		c.logger.Warnw("SubscribeRTP: PeerConn().OnTrack: ", "name", c.Name(), "!= streamID", tr.StreamID())
		return
	}

	c.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		defer func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := c.unsubscribeAll(); err != nil {
				c.logger.Errorw("unsubscribeAll", "err", err, "name", c.Name())
			}
		}()
		for {
			pkt, _, err := tr.ReadRTP()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					c.logger.Errorw("SubscribeRTP: camera client", "name ", c.Name(),
						"OnTrack callback hit unexpected error from ReadRTP err:", err.Error())
				}
				c.logger.Infow("SubscribeRTP: camera client", "name ", c.Name(),
					"OnTrack callback terminating ReadRTP loop due to ", err.Error())
				return
			}

			c.mu.Lock()
			if len(c.subAndCallbackByID) == 0 {
				c.mu.Unlock()
				c.logger.Infow("SubscribeRTP: camera client", "name", c.Name(),
					"OnTrack callback terminating as the camera client no longer has any subscribers")
				return
			}

			for _, subAndCB := range c.subAndCallbackByID {
				if err := subAndCB.sub.Publish(func() error { return subAndCB.cb(pkt) }); err != nil {
					c.logger.Infow("SubscribeRTP: camera client", "name", c.Name(), "dropped an RTP packet dropped due to", "err", err.Error())
				}
			}
			c.mu.Unlock()
		}
	}, c.activeBackgroundWorkers.Done)
}

func (c *client) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	ctx, span := trace.StartSpan(ctx, "camera::client::Unsubscribe")
	defer span.End()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn.PeerConn() == nil {
		return ErrNoPeerConnection
	}

	sc, ok := c.conn.(*rdkgrpc.SharedConn)
	if !ok {
		return ErrNoSharedPeerConnection
	}
	c.logger.CInfow(ctx, "Unsubscribe called with", "name", c.Name(), "subID", id.String())

	subAndCB, ok := c.subAndCallbackByID[id]
	if !ok {
		c.logger.CWarnw(ctx, "Unsubscribe called with unknown subID ", "name", c.Name(), "subID", id.String())
		return ErrUnknownStreamSubscriptionID
	}
	subAndCB.sub.Close()
	delete(c.subAndCallbackByID, id)

	if len(c.subAndCallbackByID) == 0 {
		sc.RemoveOnTrackSub(c.Name())
		c.logger.CInfow(ctx, "Unsubscribe calling RemoveStream", "name", c.Name(), "subID", id.String())
		if _, err := c.streamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{Name: c.Name().String()}); err != nil {
			c.logger.CWarnw(ctx, "Unsubscribe RemoveStream returned err", "name", c.Name(), "subID", id.String(), "err", err)
			return err
		}
	}

	return nil
}

func (c *client) unsubscribeAll() error {
	sc, ok := c.conn.(*rdkgrpc.SharedConn)
	if !ok {
		return nil
	}
	var errAgg error
	if len(c.subAndCallbackByID) > 0 {
		for id, subAndCB := range c.subAndCallbackByID {
			c.logger.Infow("unsubscribeAll", "name", c.Name(), "subID", id.String())
			subAndCB.sub.Close()
			delete(c.subAndCallbackByID, id)
		}

		sc.RemoveOnTrackSub(c.Name())
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), unsubscribeAllRemoveStreamTimeout)
		c.logger.Infow("unsubscribeAll calling RemoveStream", "name", c.Name())
		_, err := c.streamClient.RemoveStream(timeoutCtx, &streampb.RemoveStreamRequest{Name: c.Name().String()})
		timeoutCancel()
		if err != nil {
			c.logger.Warnw("unsubscribeAll RemoveStream returned err", "name", c.Name(), "err", err)
			errAgg = multierr.Combine(errAgg, errors.Wrapf(err, "error calling RemoveStream with name: %s", c.Name()))
		}
	}

	return errAgg
}

func (s subAndCallbackByID) String() string {
	if len(s) == 0 {
		return "len: 0"
	}
	strIds := []string{}
	strIdsToCB := map[string]subAndCallback{}
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
