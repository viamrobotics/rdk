package imu

import (
	"context"
	"github.com/golang/protobuf/ptypes/any"
	"go.viam.com/rdk/data"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/utils/rpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"sync"
	"time"
)

// TODO: parameters
// TODO: Generalize so we hopefully don't need 80 lines of boiler plater per method...

type ReadAngularVelocityCollector struct {
	client pb.IMUServiceClient

	// TODO: I think anything that doesn't use client can be shared
	queue    chan *any.Any
	Interval time.Duration
	name     string
	lock     sync.Mutex
	target   *os.File
	done     chan bool
}

type ReadOrientationCollector struct {
	client pb.IMUServiceClient

	queue    chan *any.Any
	Interval time.Duration
	name     string
	lock     sync.Mutex
	target   *os.File
	done     chan bool
}

func NewReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) interface{} {
	return ReadAngularVelocityCollector{
		Interval: interval,
		name:     name,
		client:   pb.NewIMUServiceClient(conn),
		lock:     sync.Mutex{},
		target:   target,
		done:     make(chan bool),
		// TODO: smarter channel buffer size?
		queue: make(chan *any.Any, 10),
	}
}

func NewReadOrientationCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) interface{} {
	return ReadOrientationCollector{
		Interval: interval,
		name:     name,
		client:   pb.NewIMUServiceClient(conn),
		lock:     sync.Mutex{},
		target:   target,
		done:     make(chan bool),
		// TODO: smarter channel buffer size?
		queue: make(chan *any.Any, 10),
	}
}

func (c *ReadAngularVelocityCollector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
}

func (c *ReadAngularVelocityCollector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

func (c *ReadOrientationCollector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
}

func (c *ReadOrientationCollector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

// TODO: Smarter error handling; don't want to return/exit on every error.
func (c *ReadAngularVelocityCollector) Collect(ctx context.Context) error {
	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(c.capture)
	errs.Go(func() error {
		return data.Write(c.queue, c.target)
	})
	return errs.Wait()
}

// TODO: Smarter error handling; don't want to return/exit on every error.
func (c *ReadOrientationCollector) Collect(ctx context.Context) error {
	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(c.capture)
	errs.Go(func() error {
		return data.Write(c.queue, c.target)
	})
	return errs.Wait()
}

func (c *ReadAngularVelocityCollector) Close() {
	c.done <- true
	close(c.queue)
	close(c.done)
}

func (c *ReadOrientationCollector) Close() {
	c.done <- true
	close(c.queue)
	close(c.done)
}

func (c *ReadAngularVelocityCollector) capture() error {
	ticker := time.NewTicker(c.Interval)
	for {
		select {
		case <-c.done:
			return nil
		case <-ticker.C:
			req := pb.ReadAngularVelocityRequest{Name: c.name}
			// TODO: what should context be here?
			resp, err := c.client.ReadAngularVelocity(context.TODO(), &req)
			if err != nil {
				return err
			}
			a, err := anypb.New(resp)
			if err != nil {
				return err
			}
			c.queue <- a
		}
	}
}

func (c *ReadOrientationCollector) capture() error {
	ticker := time.NewTicker(c.Interval)
	for {
		select {
		case <-c.done:
			return nil
		case <-ticker.C:
			req := pb.ReadOrientationRequest{Name: c.name}
			// TODO: what should context be here?
			resp, err := c.client.ReadOrientation(context.TODO(), &req)
			if err != nil {
				return err
			}
			a, err := anypb.New(resp)
			if err != nil {
				return err
			}
			c.queue <- a
		}
	}
}
