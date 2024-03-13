// Package board contains a gRPC based board client.
package board

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements BoardServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	client pb.BoardServiceClient
	logger logging.Logger

	info           boardInfo
	cachedStatus   *commonpb.BoardStatus
	cachedStatusMu sync.Mutex

	streamCancel  context.CancelFunc
	streamRunning bool
	streamReady   bool
	streamMu      sync.Mutex
	mu            sync.RWMutex

	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers context.CancelFunc
	extra                   *structpb.Struct
}

type boardInfo struct {
	name                  string
	analogReaderNames     []string
	digitalInterruptNames []string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Board, error) {
	info := boardInfo{name: name.ShortName()}
	bClient := pb.NewBoardServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		client: bClient,
		logger: logger,
		info:   info,
	}
	if err := c.refresh(ctx); err != nil {
		c.logger.CWarn(ctx, err)
	}
	return c, nil
}

func (c *client) AnalogReaderByName(name string) (AnalogReader, bool) {
	return &analogReaderClient{
		client:           c,
		boardName:        c.info.name,
		analogReaderName: name,
	}, true
}

func (c *client) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	return &digitalInterruptClient{
		client:               c,
		boardName:            c.info.name,
		digitalInterruptName: name,
	}, true
}

func (c *client) GPIOPinByName(name string) (GPIOPin, error) {
	return &gpioPinClient{
		client:    c,
		boardName: c.info.name,
		pinName:   name,
	}, nil
}

func (c *client) AnalogReaderNames() []string {
	if c.getCachedStatus() == nil {
		c.logger.Debugw("no cached status")
		return []string{}
	}
	return copyStringSlice(c.info.analogReaderNames)
}

func (c *client) DigitalInterruptNames() []string {
	if c.getCachedStatus() == nil {
		c.logger.Debugw("no cached status")
		return []string{}
	}
	return copyStringSlice(c.info.digitalInterruptNames)
}

// Status uses the cached status or a newly fetched board status to return the state
// of the board.
func (c *client) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	if status := c.getCachedStatus(); status != nil {
		return status, nil
	}

	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Status(ctx, &pb.StatusRequest{Name: c.info.name, Extra: ext})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (c *client) refresh(ctx context.Context) error {
	status, err := c.status(ctx)
	if err != nil {
		return errors.Wrap(err, "status call failed")
	}
	c.storeStatus(status)

	c.info.analogReaderNames = []string{}
	for name := range status.Analogs {
		c.info.analogReaderNames = append(c.info.analogReaderNames, name)
	}
	c.info.digitalInterruptNames = []string{}
	for name := range status.DigitalInterrupts {
		c.info.digitalInterruptNames = append(c.info.digitalInterruptNames, name)
	}

	return nil
}

// storeStatus atomically stores the status response.
func (c *client) storeStatus(status *commonpb.BoardStatus) {
	c.cachedStatusMu.Lock()
	defer c.cachedStatusMu.Unlock()
	c.cachedStatus = status
}

// getCachedStatus atomically gets the cached status response.
func (c *client) getCachedStatus() *commonpb.BoardStatus {
	c.cachedStatusMu.Lock()
	defer c.cachedStatusMu.Unlock()
	return c.cachedStatus
}

// status gets the latest status from the server.
func (c *client) status(ctx context.Context) (*commonpb.BoardStatus, error) {
	resp, err := c.client.Status(ctx, &pb.StatusRequest{Name: c.info.name})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (c *client) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	var dur *durationpb.Duration
	if duration != nil {
		dur = durationpb.New(*duration)
	}
	_, err := c.client.SetPowerMode(ctx, &pb.SetPowerModeRequest{Name: c.info.name, PowerMode: mode, Duration: dur})
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.info.name, cmd)
}

// WriteAnalog writes the analog value to the specified pin.
func (c *client) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.WriteAnalog(ctx, &pb.WriteAnalogRequest{
		Name:  c.info.name,
		Pin:   pin,
		Value: value,
		Extra: ext,
	})

	return err
}

// analogReaderClient satisfies a gRPC based board.AnalogReader. Refer to the interface
// for descriptions of its methods.
type analogReaderClient struct {
	*client
	boardName        string
	analogReaderName string
}

func (arc *analogReaderClient) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	resp, err := arc.client.client.ReadAnalogReader(ctx, &pb.ReadAnalogReaderRequest{
		BoardName:        arc.boardName,
		AnalogReaderName: arc.analogReaderName,
		Extra:            ext,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Value), nil
}

// digitalInterruptClient satisfies a gRPC based board.DigitalInterrupt. Refer to the
// interface for descriptions of its methods.
type digitalInterruptClient struct {
	*client
	boardName            string
	digitalInterruptName string
	callbacks            []chan Tick
}

func (dic *digitalInterruptClient) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	resp, err := dic.client.client.GetDigitalInterruptValue(ctx, &pb.GetDigitalInterruptValueRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
		Extra:                ext,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (dic *digitalInterruptClient) Tick(ctx context.Context, high bool, nanoseconds uint64) error {
	tick := Tick{Name: dic.digitalInterruptName, High: high, TimestampNanosec: nanoseconds}
	for _, ch := range dic.callbacks {
		ch <- tick
	}

	return nil
}

func (dic *digitalInterruptClient) AddCallback(ch chan Tick) {
	dic.callbacks = append(dic.callbacks, ch)
}

func (dic *digitalInterruptClient) RemoveCallback(ch chan Tick) {
	dic.mu.Lock()
	defer dic.mu.Unlock()
	for id := range dic.callbacks {
		if dic.callbacks[id] == ch {
			// To remove this item, we replace it with the last item in the list, then truncate the
			// list by 1.
			dic.callbacks[id] = dic.callbacks[len(dic.callbacks)-1]
			dic.callbacks = dic.callbacks[:len(dic.callbacks)-1]
			break
		}
	}
}

func (c *client) StreamTicks(ctx context.Context, interrupts []string, ch chan Tick, extra map[string]interface{}) error {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	c.extra = ext

	// Start the stream
	c.streamRunning = true
	c.activeBackgroundWorkers.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	c.cancelBackgroundWorkers = cancel
	// Create a background go routine that connects to the server's stream
	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundWorkers.Done()
		c.connectTickStream(ctx, interrupts, ch)
	})
	c.mu.RLock()
	ready := c.streamReady
	c.mu.RUnlock()

	// wait until the stream is ready to return
	for !ready {
		c.mu.RLock()
		ready = c.streamReady
		c.mu.RUnlock()
		// wait for 50 ms before checking again
		if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
			return ctx.Err()
		}
	}
	return nil
}

func (c *client) connectTickStream(ctx context.Context, interrupts []string, ch chan Tick) {
	defer func() {
		c.streamMu.Lock()
		defer c.streamMu.Unlock()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.streamCancel = nil
		c.streamRunning = false
		c.streamReady = false
	}()

	streamCtx, cancel := context.WithCancel(ctx)
	c.streamCancel = cancel

	for {
		c.mu.Lock()
		c.streamReady = false
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		default:
		}

		req := &pb.StreamTicksRequest{
			Name:     c.info.name,
			PinNames: interrupts,
		}

		stream, err := c.client.StreamTicks(streamCtx, req)
		if err != nil {
			c.logger.CError(ctx, err)
			if utils.SelectContextOrWait(ctx, 3*time.Second) {
				continue
			} else {
				return
			}
		}

		// repeatly receive from the stream
		for {
			select {
			case <-ctx.Done():
				c.logger.CError(ctx, err)
				return
			default:
			}

			c.mu.Lock()
			c.streamReady = true
			c.mu.Unlock()
			streamResp, err := stream.Recv()
			if err != nil && streamResp == nil {
				// only debug log the context canceled error
				c.logger.Debug(err)
				c.streamCancel()
				return
			}
			if err != nil {
				c.logger.CError(ctx, err)
			} else {
				// If there is a response with a tick, send it to the channel
				tick := Tick{
					Name:             streamResp.PinName,
					High:             streamResp.High,
					TimestampNanosec: streamResp.Time,
				}
				ch <- tick
			}
		}
	}
}

// Close cleanly closes the underlying connections.
func (dic *digitalInterruptClient) Close(ctx context.Context) error {
	if dic.cancelBackgroundWorkers != nil {
		dic.cancelBackgroundWorkers()
		dic.cancelBackgroundWorkers = nil
	}
	dic.activeBackgroundWorkers.Wait()
	return nil
}

// gpioPinClient satisfies a gRPC based board.GPIOPin. Refer to the interface
// for descriptions of its methods.
type gpioPinClient struct {
	*client
	boardName string
	pinName   string
}

func (gpc *gpioPinClient) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = gpc.client.client.SetGPIO(ctx, &pb.SetGPIORequest{
		Name:  gpc.boardName,
		Pin:   gpc.pinName,
		High:  high,
		Extra: ext,
	})
	return err
}

func (gpc *gpioPinClient) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	resp, err := gpc.client.client.GetGPIO(ctx, &pb.GetGPIORequest{
		Name:  gpc.boardName,
		Pin:   gpc.pinName,
		Extra: ext,
	})
	if err != nil {
		return false, err
	}
	return resp.High, nil
}

func (gpc *gpioPinClient) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return math.NaN(), err
	}
	resp, err := gpc.client.client.PWM(ctx, &pb.PWMRequest{
		Name:  gpc.boardName,
		Pin:   gpc.pinName,
		Extra: ext,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.DutyCyclePct, nil
}

func (gpc *gpioPinClient) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = gpc.client.client.SetPWM(ctx, &pb.SetPWMRequest{
		Name:         gpc.boardName,
		Pin:          gpc.pinName,
		DutyCyclePct: dutyCyclePct,
		Extra:        ext,
	})
	return err
}

func (gpc *gpioPinClient) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	resp, err := gpc.client.client.PWMFrequency(ctx, &pb.PWMFrequencyRequest{
		Name:  gpc.boardName,
		Pin:   gpc.pinName,
		Extra: ext,
	})
	if err != nil {
		return 0, err
	}
	return uint(resp.FrequencyHz), nil
}

func (gpc *gpioPinClient) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = gpc.client.client.SetPWMFrequency(ctx, &pb.SetPWMFrequencyRequest{
		Name:        gpc.boardName,
		Pin:         gpc.pinName,
		FrequencyHz: uint64(freqHz),
		Extra:       ext,
	})
	return err
}

// copyStringSlice is a helper to simply copy a string slice
// so that no one mutates it.
func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}
