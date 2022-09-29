package control

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type filterType string

const (
	filterFIRMovingAverage  filterType = "filterFIRMovingAverage"
	filterFIRWindowedSinc   filterType = "filterFIRWindowedSinc"
	filterIIRButterworth    filterType = "filterIIRButterworth"
	filterIIRChebyshevTypeI filterType = "filterIIRChebyshevTypeI"
)

type filter interface {
	Reset() error
	Next(x float64) (float64, bool)
}

type filterStruct struct {
	mu     sync.Mutex
	cfg    BlockConfig
	filter filter
	y      []Signal
	logger golog.Logger
}

func newFilter(config BlockConfig, logger golog.Logger) (Block, error) {
	f := &filterStruct{cfg: config, logger: logger}
	if err := f.initFilter(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *filterStruct) initFilter() error {
	if !f.cfg.Attribute.Has("type") {
		return errors.Errorf("filter %s config should have a type field", f.cfg.Name)
	}
	f.y = make([]Signal, 1)
	f.y[0] = makeSignal(f.cfg.Name)
	fType := f.cfg.Attribute.String("type")
	switch filterType(fType) {
	case filterFIRMovingAverage:
		if !f.cfg.Attribute.Has("filter_size") {
			return errors.Errorf("filter %s of type %s should have a filter_size field", f.cfg.Name, fType)
		}
		flt := movingAverageFilter{
			filterSize: f.cfg.Attribute.Int("filter_size", 0),
		}
		f.filter = &flt
		return f.filter.Reset()
	case filterFIRWindowedSinc:
		if !f.cfg.Attribute.Has("fs") {
			return errors.Errorf("filter %s of type %s should have a fs field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("fc") {
			return errors.Errorf("filter %s of type %s should have a fc field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("kernel_size") {
			return errors.Errorf("filter %s of type %s should have a kernel_size field", f.cfg.Name, fType)
		}
		flt := firWindowedSinc{
			smpFreq:    f.cfg.Attribute.Float64("fs", 0.0),
			cutOffFreq: f.cfg.Attribute.Float64("fc", 0.0),
			kernelSize: f.cfg.Attribute.Int("kernel_size", 0),
		}
		f.filter = &flt
		return f.filter.Reset()
	case filterIIRButterworth:
		if !f.cfg.Attribute.Has("fs") {
			return errors.Errorf("filter %s of type %s should have a fs field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("fc") {
			return errors.Errorf("filter %s of type %s should have a fc field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("gp") {
			return errors.Errorf("filter %s of type %s should have a gp field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("gs") {
			return errors.Errorf("filter %s of type %s should have a gs field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("order") {
			return errors.Errorf("filter %s of type %s should have a order field", f.cfg.Name, fType)
		}
		flt := iirFilter{
			smpFreq:    f.cfg.Attribute.Float64("fs", 0.0),
			n:          f.cfg.Attribute.Int("order", 0.0),
			cutOffFreq: f.cfg.Attribute.Float64("fc", 0.0),
			ripple:     0.0,
			fltType:    f.cfg.Attribute.String("filter_type"),
		}
		f.filter = &flt
		return f.filter.Reset()
	case filterIIRChebyshevTypeI:
		if !f.cfg.Attribute.Has("fs") {
			return errors.Errorf("filter %s of type %s should have a fs field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("fc") {
			return errors.Errorf("filter %s of type %s should have a fc field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("gp") {
			return errors.Errorf("filter %s of type %s should have a gp field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("gs") {
			return errors.Errorf("filter %s of type %s should have a gs field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("order") {
			return errors.Errorf("filter %s of type %s should have a order field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("ripple") {
			return errors.Errorf("filter %s of type %s should have a ripple field", f.cfg.Name, fType)
		}
		if !f.cfg.Attribute.Has("filter_type") {
			return errors.Errorf("filter %s of type %s should have a filter_type field", f.cfg.Name, fType)
		}
		flt := iirFilter{
			smpFreq:    f.cfg.Attribute.Float64("fs", 0.0),
			n:          f.cfg.Attribute.Int("order", 0.0),
			cutOffFreq: f.cfg.Attribute.Float64("fc", 0.0),
			ripple:     f.cfg.Attribute.Float64("ripple", 0.0),
			fltType:    f.cfg.Attribute.String("filter_type"),
		}
		f.filter = &flt
		return f.filter.Reset()
	default:
		return errors.Errorf("unsupported filter type %s for filter %s", fType, f.cfg.Name)
	}
}

func (f *filterStruct) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) == 1 {
		xFlt, ok := f.filter.Next(x[0].GetSignalValueAt(0))
		f.y[0].SetSignalValueAt(0, xFlt)
		return f.y, ok
	}
	return f.y, false
}

func (f *filterStruct) Reset(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.filter.Reset()
}

func (f *filterStruct) Config(ctx context.Context) BlockConfig {
	return f.cfg
}

func (f *filterStruct) UpdateConfig(ctx context.Context, config BlockConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg = config
	return f.initFilter()
}

func (f *filterStruct) Output(ctx context.Context) []Signal {
	return f.y
}
