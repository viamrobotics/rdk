// Package input contains a gRPC based input controller client.
package input

import (
	"context"
	"sync"
	"time"

	pb "go.viam.com/api/component/inputcontroller/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements InputControllerServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	client pb.InputControllerServiceClient
	logger logging.Logger

	name          string
	streamCancel  context.CancelFunc
	streamHUP     bool
	streamRunning bool
	streamReady   bool
	streamMu      sync.Mutex
	mu            sync.RWMutex

	closeContext            context.Context
	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers context.CancelFunc
	callbackWait            sync.WaitGroup
	callbacks               map[Control]map[EventType]ControlFunction
	extra                   *structpb.Struct
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Controller, error) {
	c := pb.NewInputControllerServiceClient(conn)
	return &client{
		Named:        name.PrependRemote(remoteName).AsNamed(),
		name:         name.ShortName(),
		client:       c,
		logger:       logger,
		closeContext: ctx,
	}, nil
}

func (c *client) Controls(ctx context.Context, extra map[string]interface{}) ([]Control, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetControls(ctx, &pb.GetControlsRequest{
		Controller: c.name,
		Extra:      ext,
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

func (c *client) Events(ctx context.Context, extra map[string]interface{}) (map[Control]Event, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEvents(ctx, &pb.GetEventsRequest{
		Controller: c.name,
		Extra:      ext,
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

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (c *client) TriggerEvent(ctx context.Context, event Event, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	eventMsg := &pb.Event{
		Time:    timestamppb.New(event.Time),
		Event:   string(event.Event),
		Control: string(event.Control),
		Value:   event.Value,
	}

	_, err = c.client.TriggerEvent(ctx, &pb.TriggerEventRequest{
		Controller: c.name,
		Event:      eventMsg,
		Extra:      ext,
	})

	return err
}

func (c *client) RegisterControlCallback(
	ctx context.Context,
	control Control,
	triggers []EventType,
	ctrlFunc ControlFunction,
	extra map[string]interface{},
) error {
	c.mu.Lock()
	if c.callbacks == nil {
		c.callbacks = make(map[Control]map[EventType]ControlFunction)
	}

	if _, ok := c.callbacks[control]; !ok {
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
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	c.extra = ext
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
		closeContext, cancel := context.WithCancel(c.closeContext)
		c.cancelBackgroundWorkers = cancel
		utils.PanicCapturingGo(func() {
			defer c.activeBackgroundWorkers.Done()
			c.connectStream(closeContext)
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
		req := &pb.StreamEventsRequest{
			Controller: c.name,
			Extra:      c.extra,
		}

		for control, v := range c.callbacks {
			outEvent := &pb.StreamEventsRequest_Events{
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

		stream, err := c.client.StreamEvents(streamCtx, req)
		if err != nil {
			c.logger.CError(ctx, err)
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

			if err != nil && streamResp == nil {
				c.mu.RLock()
				hup := c.streamHUP
				c.mu.RUnlock()
				if hup {
					break
				}
				c.sendConnectionStatus(ctx, false)
				if utils.SelectContextOrWait(ctx, 3*time.Second) {
					c.logger.CError(ctx, err)
					break
				} else {
					return
				}
			}
			if err != nil {
				c.logger.CError(ctx, err)
			}
			eventIn := streamResp.Event
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

// Close cleanly closes the underlying connections.
func (c *client) Close(ctx context.Context) error {
	if c.cancelBackgroundWorkers != nil {
		c.cancelBackgroundWorkers()
		c.cancelBackgroundWorkers = nil
	}
	c.activeBackgroundWorkers.Wait()
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
