package perf

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/event"
	"go.opencensus.io/trace"

	"go.viam.com/utils/perf/statz"
	"go.viam.com/utils/perf/statz/units"
)

// from https://github.com/entropyx/mongo-opencensus

type config struct {
	sampler trace.Sampler
}

// MongoDBMonitorOption represents an option that can be passed to NewMongoDBMonitor.
type MongoDBMonitorOption func(*config)

// WithMongoDBMonitorSampler set a sampler for all started spans.
func WithMongoDBMonitorSampler(sampler trace.Sampler) MongoDBMonitorOption {
	return func(cfg *config) {
		cfg.sampler = sampler
	}
}

type spanKey struct {
	ConnectionID string
	RequestID    int64
}

type monitor struct {
	sync.Mutex
	spans map[spanKey]*trace.Span
	cfg   *config
}

func (m *monitor) Started(ctx context.Context, evt *event.CommandStartedEvent) {
	connString := connectionString(evt)
	attrs := []trace.Attribute{
		trace.StringAttribute("db.system", "mongodb"),
		trace.StringAttribute("db.name", evt.DatabaseName),
		trace.StringAttribute("db.operation", evt.CommandName),
		trace.StringAttribute("db.connection_string", connString),
	}
	var collStr string
	if cmdVal, err := evt.Command.LookupErr(evt.CommandName); err == nil {
		if str, ok := cmdVal.StringValueOK(); ok {
			collStr = str
			attrs = append(attrs, trace.StringAttribute("db.mongodb.collection", collStr))
		}
	}
	var spanName string
	if collStr == "" {
		spanName = fmt.Sprintf("%s::%s", evt.DatabaseName, evt.CommandName)
	} else {
		spanName = fmt.Sprintf("%s::%s::%s", evt.DatabaseName, collStr, evt.CommandName)
	}
	_, span := trace.StartSpan(ctx, spanName, trace.WithSampler(m.cfg.sampler))
	span.AddAttributes(attrs...)
	key := spanKey{
		ConnectionID: evt.ConnectionID,
		RequestID:    evt.RequestID,
	}
	m.Lock()
	m.spans[key] = span
	m.Unlock()
}

func (m *monitor) Succeeded(ctx context.Context, evt *event.CommandSucceededEvent) {
	m.Finished(&evt.CommandFinishedEvent, nil)
}

func (m *monitor) Failed(ctx context.Context, evt *event.CommandFailedEvent) {
	m.Finished(&evt.CommandFinishedEvent, fmt.Errorf("%s", evt.Failure))
}

func (m *monitor) Finished(evt *event.CommandFinishedEvent, err error) {
	key := spanKey{
		ConnectionID: evt.ConnectionID,
		RequestID:    evt.RequestID,
	}
	m.Lock()
	span, ok := m.spans[key]
	if ok {
		delete(m.spans, key)
	}
	m.Unlock()
	if !ok {
		return
	}
	if err != nil {
		span.AddAttributes(trace.StringAttribute("error.msg", err.Error()))
	}
	span.End()
}

// NewMongoDBMonitor creates a new mongodb event CommandMonitor.
func NewMongoDBMonitor(opts ...MongoDBMonitorOption) *event.CommandMonitor {
	cfg := new(config)
	for _, opt := range opts {
		opt(cfg)
	}
	m := &monitor{
		spans: make(map[spanKey]*trace.Span),
		cfg:   cfg,
	}
	return &event.CommandMonitor{
		Started:   m.Started,
		Succeeded: m.Succeeded,
		Failed:    m.Failed,
	}
}

func connectionString(evt *event.CommandStartedEvent) string {
	hostname := evt.ConnectionID
	port := "27017"
	if idx := strings.IndexByte(hostname, '['); idx >= 0 {
		hostname = hostname[:idx]
	}
	if idx := strings.IndexByte(hostname, ':'); idx >= 0 {
		port = hostname[idx+1:]
		hostname = hostname[:idx]
	}
	return hostname + ":" + port
}

// NewMongoDBPoolMonitor creates a new mongodb pool event PoolMonitor.
func NewMongoDBPoolMonitor() *event.PoolMonitor {
	var totalWaitingToCheckOut atomic.Int64
	var totalCheckedOut atomic.Int64
	var totalCreated atomic.Int64
	return &event.PoolMonitor{
		Event: func(e *event.PoolEvent) {
			switch e.Type {
			case event.GetStarted:
				totalWaitingToCheckOut.Add(1)
				mongodbConnectionPoolStates.Set(e.Address, "total_waiting_to_check_out", totalWaitingToCheckOut.Load())
			case event.GetSucceeded:
				totalCheckedOut.Add(1)
				totalWaitingToCheckOut.Add(-1)
				mongodbConnectionPoolStates.Set(e.Address, "total_checked_out", totalCheckedOut.Load())
				mongodbConnectionPoolStates.Set(e.Address, "total_waiting_to_check_out", totalWaitingToCheckOut.Load())
			case event.GetFailed:
				totalWaitingToCheckOut.Add(-1)
				mongodbConnectionPoolStates.Set(e.Address, "total_waiting_to_check_out", totalWaitingToCheckOut.Load())
			case event.ConnectionReturned:
				totalCheckedOut.Add(-1)
				mongodbConnectionPoolStates.Set(e.Address, "total_checked_out", totalCheckedOut.Load())
			case event.ConnectionCreated:
				totalCreated.Add(1)
				mongodbConnectionPoolStates.Set(e.Address, "total_created", totalCreated.Load())
			case event.ConnectionClosed:
				totalCreated.Add(-1)
				mongodbConnectionPoolStates.Set(e.Address, "total_created", totalCreated.Load())
			}
		},
	}
}

var mongodbConnectionPoolStates = statz.NewGauge2[string, string]("mongodb/connections", statz.MetricConfig{
	Description: "The number of waiting requests for connection check out.",
	Unit:        units.Dimensionless,
	Labels: []statz.Label{
		{Name: "connection_string", Description: "MongoDB Connection String"},
		{Name: "state", Description: "Pool State"},
	},
})
