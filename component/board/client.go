// Package board contains a gRPC based board client.
package board

import (
	"context"
	"runtime/debug"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
)

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

// Close cleanly closes the underlying connections
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is an Board client
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Board, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(conn rpc.ClientConn, name string, logger golog.Logger) Board {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Board {
	return &client{sc, name}
}

// SPIByName may need to be implemented
func (c *client) SPIByName(name string) (SPI, bool) {
	return nil, false
}

// I2CByName may need to be implemented
func (c *client) I2CByName(name string) (I2C, bool) {
	return nil, false
}

func (c *client) AnalogReaderByName(name string) (AnalogReader, bool) {
	return &analogReaderClient{
		serviceClient:    c.serviceClient,
		boardName:        c.name,
		analogReaderName: name,
	}, true
}

func (c *client) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	return &digitalInterruptClient{
		serviceClient:        c.serviceClient,
		boardName:            c.name,
		digitalInterruptName: name,
	}, true
}

func (c *client) GPIOSet(ctx context.Context, pin string, high bool) error {
	_, err := c.client.GPIOSet(ctx, &pb.BoardServiceGPIOSetRequest{
		Name: c.name,
		Pin:  pin,
		High: high,
	})
	return err
}

func (c *client) GPIOGet(ctx context.Context, pin string) (bool, error) {
	resp, err := c.client.GPIOGet(ctx, &pb.BoardServiceGPIOGetRequest{
		Name: c.name,
		Pin:  pin,
	})
	if err != nil {
		return false, err
	}
	return resp.High, nil
}

func (c *client) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	_, err := c.client.PWMSet(ctx, &pb.BoardServicePWMSetRequest{
		Name:      c.name,
		Pin:       pin,
		DutyCycle: uint32(dutyCycle),
	})
	return err
}

func (c *client) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	_, err := c.client.PWMSetFrequency(ctx, &pb.BoardServicePWMSetFrequencyRequest{
		Name:      c.name,
		Pin:       pin,
		Frequency: uint64(freq),
	})
	return err
}

func (c *client) SPINames() []string {
	return copyStringSlice(c.SPINames())
}

func (c *client) I2CNames() []string {
	return copyStringSlice(c.I2CNames())
}

func (c *client) AnalogReaderNames() []string {
	return copyStringSlice(c.AnalogReaderNames())
}

func (c *client) DigitalInterruptNames() []string {
	return copyStringSlice(c.DigitalInterruptNames())
}

func (c *client) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	// TODO(maximpertsov): can we rely on caching in main client?
	// if status := bc.rc.getCachedStatus(); status != nil {
	// 	boardStatus, ok := status.Boards[bc.info.name]
	// 	if !ok {
	// 		return nil, errors.Errorf("no board with name (%s)", bc.info.name)
	// 	}
	// 	return boardStatus, nil
	// }
	resp, err := c.client.Status(ctx, &pb.BoardServiceStatusRequest{Name: c.name})
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

func (c *client) ModelAttributes() ModelAttributes {
	return ModelAttributes{Remote: true}
}

// analogReaderClient satisfies a gRPC based board.AnalogReader. Refer to the interface
// for descriptions of its methods.
type analogReaderClient struct {
	*serviceClient
	boardName        string
	analogReaderName string
}

func (arc *analogReaderClient) Read(ctx context.Context) (int, error) {
	resp, err := arc.client.AnalogReaderRead(ctx, &pb.BoardServiceAnalogReaderReadRequest{
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

func (dic *digitalInterruptClient) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	resp, err := dic.client.DigitalInterruptConfig(ctx, &pb.BoardServiceDigitalInterruptConfigRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
	})
	if err != nil {
		return DigitalInterruptConfig{}, err
	}
	return DigitalInterruptConfigFromProto(resp.Config), nil
}

// DigitalInterruptConfigFromProto converts a proto based digital interrupt config to the
// codebase specific version.
func DigitalInterruptConfigFromProto(config *pb.DigitalInterruptConfig) DigitalInterruptConfig {
	return DigitalInterruptConfig{
		Name:    config.Name,
		Pin:     config.Pin,
		Type:    config.Type,
		Formula: config.Formula,
	}
}

func (dic *digitalInterruptClient) Value(ctx context.Context) (int64, error) {
	resp, err := dic.client.DigitalInterruptValue(ctx, &pb.BoardServiceDigitalInterruptValueRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
	})
	if err != nil {
		return 0, err
	}
	return resp.Value, nil
}

func (dic *digitalInterruptClient) Tick(ctx context.Context, high bool, nanos uint64) error {
	_, err := dic.client.DigitalInterruptTick(ctx, &pb.BoardServiceDigitalInterruptTickRequest{
		BoardName:            dic.boardName,
		DigitalInterruptName: dic.digitalInterruptName,
		High:                 high,
		Nanos:                nanos,
	})
	return err
}

func (dic *digitalInterruptClient) AddCallback(c chan bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (dic *digitalInterruptClient) AddPostProcessor(pp PostProcessor) {
	debug.PrintStack()
	panic(errUnimplemented)
}

// Close cleanly closes the underlying connections. No methods should be called on the
// board after this
func (c *client) Close() error {
	return c.serviceClient.Close()
}

// TODO(maximpertsov): copied from common client - export into utils (or export from
// common client)?
// copyStringSlice is a helper to simply copy a string slice
// so that no one mutates it.
func copyStringSlice(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// TODO(maximpertsov): copied from common client - export into utils (or export from
// common client)?
// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")
