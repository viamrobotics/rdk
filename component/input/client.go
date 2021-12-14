// Package input contains a gRPC based input controller client.
package input

import (
	"context"
	"sync"
	"time"

	"go.viam.com/utils"
	rpcclient "go.viam.com/utils/rpc/client"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/grpc"
	pb "go.viam.com/core/proto/api/component/v1"
)

// serviceClient is a client satisfies the proto contract.
type serviceClient struct {
	conn   dialer.ClientConn
	client pb.InputControllerServiceClient
	logger golog.Logger
}

// newServiceClient constructs a new serviceClient that is served at the given address.
func newServiceClient(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn dialer.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewInputControllerServiceClient(conn)
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

// client is an input controller client
type client struct {
	*serviceClient
	name          string
	streamCancel  context.CancelFunc
	streamHUP     bool
	streamRunning bool
	streamReady   bool
	streamMu      sync.Mutex
	mu            sync.RWMutex

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	callbackWait            sync.WaitGroup
	callbacks               map[Control]map[EventType]ControlFunction
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, opts rpcclient.DialOptions, logger golog.Logger) (Controller, error) {
	sc, err := newServiceClient(ctx, address, opts, logger)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(conn dialer.ClientConn, name string, logger golog.Logger) Controller {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Controller {
	closeCtx, cancel := context.WithCancel(context.Background())
	return &client{serviceClient: sc, name: name}
}

func (c *client) Controls(ctx context.Context) ([]Control, error) {
	resp, err := c.client.Controls(ctx, &pb.InputControllerServiceControlsRequest{
		Controller: c.name,
	})
	if err != nil {
		return nil, err
	}
	var controls []Control
	for _, control := range resp.Controls {
		controls = append(controls, Control(control))
	}
	return controls, nil
}

func (c *client) LastEvents(ctx context.Context) (map[Control]Event, error) {
	resp, err := c.client.LastEvents(ctx, &pb.InputControllerServiceLastEventsRequest{
		Controller: c.name,
	})
	if err != nil {
		return nil, err
	}

	eventsOut := make(map[Control]Event)
	for _, eventIn := range resp.Events {
		eventsOut[Control(eventIn.Control)] = Event{
			Time:    eventIn.Time.AsTime(),
			Event:   EventType(eventIn.Event),
			Control: Control(eventIn.Control),
			Value:   eventIn.Value,
		}
	}
	return eventsOut, nil
}

// InjectEvent allows directly sending an Event (such as a button press) from external code
func (c *client) InjectEvent(ctx context.Context, event Event) error {
	eventMsg := &pb.InputControllerServiceEvent{
		Time:    timestamppb.New(event.Time),
		Event:   string(event.Event),
		Control: string(event.Control),
		Value:   event.Value,
	}

	_, err := c.client.InjectEvent(ctx, &pb.InputControllerServiceInjectEventRequest{
		Controller: c.name,
		Event:      eventMsg,
	})

	return err
}

func (c *client) RegisterControlCallback(ctx context.Context, control Control, triggers []EventType, ctrlFunc ControlFunction) error {
	c.mu.Lock()
	if c.callbacks == nil {
		c.callbacks = make(map[Control]map[EventType]ControlFunction)
	}

	_, ok := c.callbacks[control]
	if !ok {
		c.callbacks[control] = make(map[EventType]ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == ButtonChange {
			c.callbacks[control][ButtonRelease] = ctrlFunc
			c.callbacks[control][ButtonPress] = ctrlFunc
		} else {
			c.callbacks[control][trigger] = ctrlFunc
		}
	}
	c.mu.Unlock()

	// We want to start one and only one connectStream()
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	if c.streamRunning {
		for !c.streamReady {
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		}
		c.streamHUP = true
		c.streamReady = false
		c.streamCancel()
	} else {
		c.streamRunning = true
		c.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer c.activeBackgroundWorkers.Done()
			c.connectStream(c.closeContext)
		})
		c.mu.RLock()
		ready := c.streamReady
		c.mu.RUnlock()
		for !ready {
			c.mu.RLock()
			ready = c.streamReady
			c.mu.RUnlock()
			if !utils.SelectContextOrWait(ctx, 50*time.Millisecond) {
				return ctx.Err()
			}
		}
	}

	return nil
}

func (c *client) connectStream(ctx context.Context) {
	defer func() {
		c.streamMu.Lock()
		defer c.streamMu.Unlock()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.streamCancel = nil
		c.streamRunning = false
		c.streamHUP = false
		c.streamReady = false
		c.callbackWait.Wait()
	}()

	// Will retry on connection errors and disconnects
	for {
		c.mu.Lock()
		c.streamReady = false
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		default:
		}

		var haveCallbacks bool
		c.mu.RLock()
		req := &pb.InputControllerServiceEventStreamRequest{
			Controller: c.name,
		}

		for control, v := range c.callbacks {
			outEvent := &pb.InputControllerServiceEventStreamRequest_Events{
				Control: string(control),
			}

			for event, ctrlFunc := range v {
				if ctrlFunc != nil {
					haveCallbacks = true
					outEvent.Events = append(outEvent.Events, string(event))
				} else {
					outEvent.CancelledEvents = append(outEvent.CancelledEvents, string(event))
				}
			}
			req.Events = append(req.Events, outEvent)
		}
		c.mu.RUnlock()

		if !haveCallbacks {
			return
		}

		streamCtx, cancel := context.WithCancel(ctx)
		c.streamCancel = cancel

		stream, err := c.client.EventStream(streamCtx, req)
		if err != nil {
			c.logger.Error(err)
			if utils.SelectContextOrWait(ctx, 3*time.Second) {
				continue
			} else {
				return
			}
		}

		c.mu.RLock()
		hup := c.streamHUP
		c.mu.RUnlock()
		if !hup {
			c.sendConnectionStatus(ctx, true)
		}

		// Handle the rest of the stream
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			c.mu.Lock()
			c.streamHUP = false
			c.streamReady = true
			c.mu.Unlock()
			streamResp, err := stream.Recv()
			eventIn := streamResp.Event
			if err != nil && eventIn == nil {
				c.mu.RLock()
				hup := c.streamHUP
				c.mu.RUnlock()
				if hup {
					break
				}
				c.sendConnectionStatus(ctx, false)
				if utils.SelectContextOrWait(ctx, 3*time.Second) {
					c.logger.Error(err)
					break
				} else {
					return
				}
			}
			if err != nil {
				c.logger.Error(err)
			}

			eventOut := Event{
				Time:    eventIn.Time.AsTime(),
				Event:   EventType(eventIn.Event),
				Control: Control(eventIn.Control),
				Value:   eventIn.Value,
			}
			c.execCallback(ctx, eventOut)
		}
	}
}

func (c *client) sendConnectionStatus(ctx context.Context, connected bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	evType := Disconnect
	now := time.Now()
	if connected {
		evType = Connect
	}

	for control := range c.callbacks {
		eventOut := Event{
			Time:    now,
			Event:   evType,
			Control: control,
			Value:   0,
		}
		c.execCallback(ctx, eventOut)
	}
}

func (c *client) execCallback(ctx context.Context, event Event) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	callbackMap, ok := c.callbacks[event.Control]
	if !ok {
		return
	}

	callback, ok := callbackMap[event.Event]
	if ok && callback != nil {
		c.callbackWait.Add(1)
		utils.PanicCapturingGo(func() {
			defer c.callbackWait.Done()
			callback(ctx, event)
		})
	}
	callbackAll, ok := callbackMap[AllEvents]
	if ok && callbackAll != nil {
		c.callbackWait.Add(1)
		utils.PanicCapturingGo(func() {
			defer c.callbackWait.Done()
			callbackAll(ctx, event)
		})
	}
}

// Close cleanly closes the underlying connections
func (c *client) Close() error {
	if c.cancelBackgroundWorkers != nil {
		c.cancelBackgroundWorkers()
		c.cancelBackgroundWorkers = nil
	}
	c.activeBackgroundWorkers.Wait()
	return c.serviceClient.Close()
}
