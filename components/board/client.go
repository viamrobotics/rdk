// Package board contains a gRPC based board client.
package board

import (
	"context"
	"math"
	"slices"
	"sync"
	"time"

	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/durationpb"

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

	info boardInfo

	interruptStreams []*interruptStream

	mu sync.Mutex
}

type boardInfo struct {
	name                  string
	analogNames           []string
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
	info := boardInfo{
		name:                  name.ShortName(),
		analogNames:           []string{},
		digitalInterruptNames: []string{},
	}
	bClient := pb.NewBoardServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		client: bClient,
		logger: logger,
		info:   info,
	}
	return c, nil
}

func (c *client) AnalogByName(name string) (Analog, error) {
	if !slices.Contains(c.info.analogNames, name) {
		c.info.analogNames = append(c.info.analogNames, name)
	}
	return &analogClient{
		client:     c,
		boardName:  c.info.name,
		analogName: name,
	}, nil
}

func (c *client) DigitalInterruptByName(name string) (DigitalInterrupt, error) {
	if !slices.Contains(c.info.digitalInterruptNames, name) {
		c.info.digitalInterruptNames = append(c.info.digitalInterruptNames, name)
	}
	return &digitalInterruptClient{
		client:               c,
		boardName:            c.info.name,
		digitalInterruptName: name,
	}, nil
}

func (c *client) GPIOPinByName(name string) (GPIOPin, error) {
	return &gpioPinClient{
		client:    c,
		boardName: c.info.name,
		pinName:   name,
	}, nil
}

func (c *client) AnalogNames() []string {
	if len(c.info.analogNames) == 0 {
		c.logger.Debugw("no cached analog readers")
		return []string{}
	}
	return copyStringSlice(c.info.analogNames)
}

func (c *client) DigitalInterruptNames() []string {
	if len(c.info.digitalInterruptNames) == 0 {
		c.logger.Debugw("no cached digital interrupts")
		return []string{}
	}
	return copyStringSlice(c.info.digitalInterruptNames)
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

// analogClient satisfies a gRPC based board.AnalogReader. Refer to the interface
// for descriptions of its methods.
type analogClient struct {
	*client
	boardName  string
	analogName string
}

func (ac *analogClient) Read(ctx context.Context, extra map[string]interface{}) (AnalogValue, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return AnalogValue{}, err
	}
	// the api method is named ReadAnalogReader, it is named differently than
	// the board interface functions.
	resp, err := ac.client.client.ReadAnalogReader(ctx, &pb.ReadAnalogReaderRequest{
		BoardName:        ac.boardName,
		AnalogReaderName: ac.analogName,
		Extra:            ext,
	})
	if err != nil {
		return AnalogValue{}, err
	}
	return AnalogValue{Value: int(resp.Value), Min: resp.MinRange, Max: resp.MaxRange, StepSize: resp.StepSize}, nil
}

func (ac *analogClient) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = ac.client.client.WriteAnalog(ctx, &pb.WriteAnalogRequest{
		Name:  ac.boardName,
		Pin:   ac.analogName,
		Value: int32(value),
		Extra: ext,
	})
	if err != nil {
		return err
	}
	return nil
}

// digitalInterruptClient satisfies a gRPC based board.DigitalInterrupt. Refer to the
// interface for descriptions of its methods.
type digitalInterruptClient struct {
	*client
	boardName            string
	digitalInterruptName string
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

func (dic *digitalInterruptClient) Name() string {
	return dic.digitalInterruptName
}

func (c *client) StreamTicks(ctx context.Context, interrupts []DigitalInterrupt, ch chan Tick,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	stream := &interruptStream{
		extra:  ext,
		client: c,
	}

	c.mu.Lock()
	c.interruptStreams = append(c.interruptStreams, stream)
	c.mu.Unlock()

	err = stream.startStream(ctx, interrupts, ch)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) removeStream(s *interruptStream) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, stream := range s.interruptStreams {
		if stream == s {
			// To remove this item, we replace it with the last item in the list, then truncate the
			// list by 1.
			s.client.interruptStreams[i] = s.client.interruptStreams[len(s.client.interruptStreams)-1]
			s.client.interruptStreams = s.client.interruptStreams[:len(s.client.interruptStreams)-1]
			break
		}
	}
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
