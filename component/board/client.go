// Package board contains a gRPC based board client.
package board

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/board/v1"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// serviceClient is a client satisfies the board.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.BoardServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewBoardServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an Board client.
type client struct {
	*serviceClient
	info boardInfo

	cachedStatus   *commonpb.BoardStatus
	cachedStatusMu *sync.Mutex
}

type boardInfo struct {
	name                  string
	spiNames              []string
	i2cNames              []string
	analogReaderNames     []string
	digitalInterruptNames []string
	gpioPinNames          []string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Board, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(ctx, sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Board {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(ctx, sc, name)
}

func clientFromSvcClient(ctx context.Context, sc *serviceClient, name string) Board {
	info := boardInfo{name: name}
	c := &client{
		serviceClient:  sc,
		info:           info,
		cachedStatusMu: &sync.Mutex{},
	}
	if err := c.refresh(ctx); err != nil {
		sc.logger.Warn(err)
	}
	return c
}

func (c *client) AnalogReaderByName(name string) (AnalogReader, bool) {
	return &analogReaderClient{
		serviceClient:    c.serviceClient,
		boardName:        c.info.name,
		analogReaderName: name,
	}, true
}

func (c *client) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	return &digitalInterruptClient{
		serviceClient:        c.serviceClient,
		boardName:            c.info.name,
		digitalInterruptName: name,
	}, true
}

func (c *client) GPIOPinByName(name string) (GPIOPin, error) {
	return &gpioPinClient{
		serviceClient: c.serviceClient,
		boardName:     c.info.name,
		pinName:       name,
	}, nil
}

func (c *client) SPINames() []string {
	if c.getCachedStatus() == nil {
		panic("no status")
	}
	return copyStringSlice(c.info.spiNames)
}

func (c *client) I2CNames() []string {
	if c.getCachedStatus() == nil {
		panic("no status")
	}
	return copyStringSlice(c.info.i2cNames)
}

func (c *client) AnalogReaderNames() []string {
	if c.getCachedStatus() == nil {
		panic("no status")
	}
	return copyStringSlice(c.info.analogReaderNames)
}

func (c *client) DigitalInterruptNames() []string {
	if c.getCachedStatus() == nil {
		panic("no status")
	}
	return copyStringSlice(c.info.digitalInterruptNames)
}

func (c *client) GPIOPinNames() []string {
	if c.getCachedStatus() == nil {
		panic("no status")
	}
	return copyStringSlice(c.info.gpioPinNames)
}

// Status uses the cached status or a newly fetched board status to return the state
// of the board.
func (c *client) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	if status := c.getCachedStatus(); status != nil {
		return status, nil
	}
	resp, err := c.client.Status(ctx, &pb.StatusRequest{Name: c.info.name})
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

func (c *client) ModelAttributes() ModelAttributes {
	return ModelAttributes{Remote: true}
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.info.name, cmd)
}

// analogReaderClient satisfies a gRPC based board.AnalogReader. Refer to the interface
// for descriptions of its methods.
type analogReaderClient struct {
	*serviceClient
	boardName        string
	analogReaderName string
}

func (arc *analogReaderClient) Read(ctx context.Context) (int, error) {
	resp, err := arc.client.ReadAnalogReader(ctx, &pb.ReadAnalogReaderRequest{
		BoardName:        arc.boardName,
		AnalogReaderName: arc.analogReaderName,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Value), nil
}

// digitalInterruptClient satisfies a gRPC based board.DigitalInterrupt. Refer to the
// interface for descriptions of its methods.
type digitalInterruptClient struct {
	*serviceClient
	boardName            string
	digitalInterruptName string
}

func (dic *digitalInterruptClient) Value(ctx context.Context) (int64, error) {
	resp, err := dic.client.GetDigitalInterruptValue(ctx, &pb.GetDigitalInterruptValueRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (dic *digitalInterruptClient) Tick(ctx context.Context, high bool, nanos uint64) error {
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) AddCallback(c chan bool) {
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) AddPostProcessor(pp PostProcessor) {
	panic(errUnimplemented)
}

// gpioPinClient satisfies a gRPC based board.GPIOPin. Refer to the interface
// for descriptions of its methods.
type gpioPinClient struct {
	*serviceClient
	boardName string
	pinName   string
}

func (gpc *gpioPinClient) Set(ctx context.Context, high bool) error {
	_, err := gpc.client.SetGPIO(ctx, &pb.SetGPIORequest{
		Name: gpc.boardName,
		Pin:  gpc.pinName,
		High: high,
	})
	return err
}

func (gpc *gpioPinClient) Get(ctx context.Context) (bool, error) {
	resp, err := gpc.client.GetGPIO(ctx, &pb.GetGPIORequest{
		Name: gpc.boardName,
		Pin:  gpc.pinName,
	})
	if err != nil {
		return false, err
	}
	return resp.High, nil
}

func (gpc *gpioPinClient) PWM(ctx context.Context) (float64, error) {
	resp, err := gpc.client.PWM(ctx, &pb.PWMRequest{
		Name: gpc.boardName,
		Pin:  gpc.pinName,
	})
	if err != nil {
		return math.NaN(), err
	}
	return resp.DutyCyclePct, nil
}

func (gpc *gpioPinClient) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	_, err := gpc.client.SetPWM(ctx, &pb.SetPWMRequest{
		Name:         gpc.boardName,
		Pin:          gpc.pinName,
		DutyCyclePct: dutyCyclePct,
	})
	return err
}

func (gpc *gpioPinClient) PWMFreq(ctx context.Context) (uint, error) {
	resp, err := gpc.client.PWMFrequency(ctx, &pb.PWMFrequencyRequest{
		Name: gpc.boardName,
		Pin:  gpc.pinName,
	})
	if err != nil {
		return 0, err
	}
	return uint(resp.FrequencyHz), nil
}

func (gpc *gpioPinClient) SetPWMFreq(ctx context.Context, freqHz uint) error {
	_, err := gpc.client.SetPWMFrequency(ctx, &pb.SetPWMFrequencyRequest{
		Name:        gpc.boardName,
		Pin:         gpc.pinName,
		FrequencyHz: uint64(freqHz),
	})
	return err
}

// Close cleanly closes the underlying connections. No methods should be called on the
// board after this.
func (c *client) Close() error {
	return c.serviceClient.Close()
}

// copyStringSlice is a helper to simply copy a string slice
// so that no one mutates it.
func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}
