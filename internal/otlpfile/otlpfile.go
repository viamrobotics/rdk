// Package otlpfile provides a way to save OTLP messages to disk instead of
// sending them over the network.
package otlpfile

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"gopkg.in/natefinch/lumberjack.v2"

	"go.viam.com/rdk/protoutils"
)

// Client is a type satisfies [otlptrace.Client] but writes to disk instead of
// the network.
type Client struct {
	mu     sync.Mutex
	logger *lumberjack.Logger
	writer *protoutils.DelimitedProtoWriter[*v1.ResourceSpans]
}

// NewClient creates a new [Client].
func NewClient(dirPath, filename string) (*Client, error) {
	logger := &lumberjack.Logger{
		Filename:   filepath.Join(dirPath, filename),
		MaxSize:    1024,
		MaxBackups: 2,
		Compress:   true,
	}
	writer := protoutils.NewDelimitedProtoWriter[*v1.ResourceSpans](logger)
	return &Client{
		logger: logger,
		writer: writer,
	}, nil
}

// Start implements [otlptrace.Client]. It is currently a noop.
func (c *Client) Start(ctx context.Context) error {
	return nil
}

// Stop implements [otlptrace.Client]. It closes underlying resources.
func (c *Client) Stop(ctx context.Context) error {
	return c.writer.Close()
}

// UploadTraces implements [otlptrace.Client]. It saves the passed protoSpans
// to disk.
func (c *Client) UploadTraces(ctx context.Context, protoSpans []*v1.ResourceSpans) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var errs error
	for _, span := range protoSpans {
		errs = errors.Join(c.writer.Append(span))
	}
	return errs
}

var _ otlptrace.Client = &Client{}
