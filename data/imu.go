package data

import (
	"context"
	pb "go.viam.com/rdk/proto/api/component/imu/v1"
	"go.viam.com/utils/rpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"os"
	"sync"
	"time"
)

// TODO: parameters
// TODO: Generalize so we hopefully don't need 80 lines of boiler plater per method...

type IMUReadAngularVelocityCollector struct {
	Interval time.Duration
	name     string
	client   pb.IMUServiceClient
	lock     sync.Mutex
	target   *os.File
	done     chan bool
	queue    chan *pb.ReadAngularVelocityResponse
}

func NewIMUReadAngularVelocityCollectorFromConn(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) interface{} {
	return IMUReadAngularVelocityCollector{
		Interval: interval,
		name:     name,
		client:   pb.NewIMUServiceClient(conn),
		lock:     sync.Mutex{},
		target:   target,
		done:     make(chan bool),
		// TODO: smarter channel buffer size?
		queue: make(chan *pb.ReadAngularVelocityResponse, 10),
	}
}

func (c *IMUReadAngularVelocityCollector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
}

func (c *IMUReadAngularVelocityCollector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

// TODO: Smarter error handling; don't want to return/exit on every error.
func (c *IMUReadAngularVelocityCollector) Collect(ctx context.Context) error {
	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(c.capture)
	errs.Go(c.write)
	return errs.Wait()
}

func (c *IMUReadAngularVelocityCollector) Close() {
	c.done <- true
	close(c.queue)
}

func (c *IMUReadAngularVelocityCollector) capture() error {
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
			c.queue <- resp
		}
	}
}

// TODO: length prefix when writing
func (c *IMUReadAngularVelocityCollector) write() error {
	for resp := range c.queue {
		bytes, err := proto.Marshal(resp)
		if err != nil {
			return err
		}
		c.target.Write(bytes)
	}
	return nil
}
