package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/component/camera/v1"
	streampb "go.viam.com/api/stream/v1"
	goutils "go.viam.com/utils"
	goprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// client implements CameraServiceClient.
type singlePacketCB func(*rtp.Packet) error
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	wg                      sync.WaitGroup
	mu                      sync.Mutex
	subs                    map[*StreamSubscription]singlePacketCB
	name                    string
	conn                    rpc.ClientConn
	client                  pb.CameraServiceClient
	streamClient            streampb.StreamServiceClient
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup
	healthyClientCh         chan struct{}
}

// var create sync.Once

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
	logger.Infof("Camera Client. PeerConn? %p\n", conn.PeerConn())
	return &client{
		Named:        name.PrependRemote(remoteName).AsNamed(),
		name:         name.ShortName(),
		conn:         conn,
		streamClient: streamClient,
		client:       c,
		subs:         map[*StreamSubscription]singlePacketCB{},
		logger:       logger,
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

// TOMORROW: NICK: TEST THIS!!!
// Figure out where to put r.Publish
// Have the client figure out if there are 0 subscribers, and not connect if so & once there is a single subscriber
// register a subscription
// Every subscriber after that should just be another reciver of the same data stream

// TODO: NICK We probably need to do the multiplexing here,
// Probably need to have the camera subscription do work here
func (c *client) SubscribeRTP(r *StreamSubscription, packetsCB PacketCallback) error {
	// TODO: Gotta add mutexes & wait for the waitgroup
	if len(c.subs) == 0 {
		// TODO: Fix this context
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second)
		defer timeoutCancel()
		c.conn.PeerConn().OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
			if len(c.subs) == 0 {
				c.logger.Debug("OnTrack called when c.subs was already empty")
				return
			}
			c.wg.Add(1)
			goutils.ManagedGo(func() {
				for {
					pkt, _, err := tr.ReadRTP()
					if err != nil {
						if err != io.EOF {
							c.logger.Fatal(err.Error())
						}
						return
					}
					if len(c.subs) == 0 {
						c.logger.Debug("OnTrack spawned function terminating as len(c.subs) == 0")
						return
					}
					for sub, cb := range c.subs {
						if err := sub.Publish(func() error { return cb(pkt) }); err != nil {
							c.logger.Debug("RTP packet dropped due to %s", err.Error())
						}
					}
				}

			}, c.wg.Done)
		})

		c.subs[r] = func(p *rtp.Packet) error { return packetsCB([]*rtp.Packet{p}) }
		r.Start()
		_, err := c.streamClient.AddStream(timeoutCtx, &streampb.AddStreamRequest{Name: c.Name().String()})
		if err != nil {
			r.Stop()
			delete(c.subs, r)
			return err
		}
	}
	return nil
}

// TODO: Unsubscribe should be able to fail
func (c *client) Unsubscribe(r *StreamSubscription) {
	// TODO: Fix this context
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second)
	defer timeoutCancel()
	_, err := c.streamClient.RemoveStream(timeoutCtx, &streampb.RemoveStreamRequest{Name: c.Name().String()})
	if err != nil {
		c.logger.Fatal(err.Error())
	}
	r.Stop()
	delete(c.subs, r)
}

func (c *client) VideoCodecStreamSource() (VideoCodecStreamSource, error) {
	if c.conn.PeerConn() != nil {
		return c, nil
	}
	return nil, errors.New("VideoCodecStreamSource unimplemented as module doesn't support peer connections")
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
					c.logger.Warn("error closing stream", err)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.healthyClientCh != nil {
		close(c.healthyClientCh)
	}
	c.activeBackgroundWorkers.Wait()
	c.healthyClientCh = nil
	return nil
}
