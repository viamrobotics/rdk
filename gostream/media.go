package gostream

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"
)

type (
	// A MediaReader is anything that can read and recycle data. It is expected
	// that reader can handle multiple reads at the same time. This would ideally only
	// happen during streaming when a specific MIME type is requested. In the future,
	// we may be able to notice multiple MIME types and either do deferred encode/decode
	// or have the reader do it for us.
	MediaReader[T any] interface {
		Read(ctx context.Context) (data T, release func(), err error)
		Close(ctx context.Context) error
	}

	// A MediaReaderFunc is a helper to turn a function into a MediaReader.
	MediaReaderFunc[T any] func(ctx context.Context) (T, func(), error)

	// A MediaStream streams media forever until closed.
	MediaStream[T any] interface {
		// Next returns the next media element in the sequence (best effort).
		// Note: This element is mutable and shared globally; it MUST be copied
		// before it is mutated.
		Next(ctx context.Context) (T, func(), error)

		// Close signals this stream is no longer needed and releases associated
		// resources.
		Close(ctx context.Context) error
	}

	// A MediaSource can produce Streams of Ts.
	MediaSource[T any] interface {
		// Stream returns a stream that makes a best effort to return consecutive media elements
		// that may have a MIME type hint dictated in the context via WithMIMETypeHint.
		Stream(ctx context.Context, errHandlers ...ErrorHandler) (MediaStream[T], error)

		// Close cleans up any associated resources with the Source (e.g. a Driver).
		Close(ctx context.Context) error
	}

	// MediaPropertyProvider providers information about a source.
	MediaPropertyProvider[U any] interface {
		MediaProperties(ctx context.Context) (U, error)
	}
)

// Read calls the underlying function to get a media.
func (mrf MediaReaderFunc[T]) Read(ctx context.Context) (T, func(), error) {
	ctx, span := trace.StartSpan(ctx, "gostream::MediaReaderFunc::Read")
	defer span.End()
	return mrf(ctx)
}

// Close does nothing.
func (mrf MediaReaderFunc[T]) Close(ctx context.Context) error {
	return nil
}

// A mediaReaderFuncNoCtx is a helper to turn a function into a MediaReader that cannot
// accept a context argument.
type mediaReaderFuncNoCtx[T any] func() (T, func(), error)

// Read calls the underlying function to get a media.
func (mrf mediaReaderFuncNoCtx[T]) Read(ctx context.Context) (T, func(), error) {
	_, span := trace.StartSpan(ctx, "gostream::MediaReaderFuncNoCtx::Read")
	defer span.End()
	return mrf()
}

// Close does nothing.
func (mrf mediaReaderFuncNoCtx[T]) Close(ctx context.Context) error {
	return nil
}

// ReadMedia gets a single media from a source. Using this has less of a guarantee
// than MediaSource.Stream that the Nth media element follows the N-1th media element.
func ReadMedia[T any](ctx context.Context, source MediaSource[T]) (T, func(), error) {
	ctx, span := trace.StartSpan(ctx, "gostream::ReadMedia")
	defer span.End()

	if reader, ok := source.(MediaReader[T]); ok {
		// more efficient if there is a direct way to read
		return reader.Read(ctx)
	}
	stream, err := source.Stream(ctx)
	var zero T
	if err != nil {
		return zero, nil, err
	}
	defer func() {
		utils.UncheckedError(stream.Close(ctx))
	}()
	return stream.Next(ctx)
}

type mediaSource[T any, U any] struct {
	driver        driver.Driver
	reader        MediaReader[T]
	props         U
	rootCancelCtx context.Context
	rootCancel    func()

	producerConsumers   map[string]*producerConsumer[T, U]
	producerConsumersMu sync.Mutex
}

type producerConsumer[T any, U any] struct {
	rootCancelCtx           context.Context
	cancelCtx               context.Context
	interestedConsumers     int64
	cancelCtxMu             *sync.RWMutex
	cancel                  func()
	mimeType                string
	activeBackgroundWorkers sync.WaitGroup
	readWrapper             MediaReader[T]
	current                 *mediaRefReleasePairWithError[T]
	currentMu               sync.RWMutex
	producerCond            *sync.Cond
	consumerCond            *sync.Cond
	condMu                  *sync.RWMutex
	errHandlers             map[*mediaStream[T, U]][]ErrorHandler
	listeners               int
	stateMu                 sync.Mutex
	listenersMu             sync.Mutex
	errHandlersMu           sync.Mutex
}

// ErrorHandler receives the error returned by a TSource.Next
// regardless of whether or not the error is nil (This allows
// for error handling logic based on consecutively retrieved errors).
type ErrorHandler func(ctx context.Context, mediaErr error)

// PropertiesFromMediaSource returns properties from underlying driver in the given MediaSource.
func PropertiesFromMediaSource[T, U any](src MediaSource[T]) ([]prop.Media, error) {
	d, err := DriverFromMediaSource[T, U](src)
	if err != nil {
		return nil, err
	}
	return d.Properties(), nil
}

// LabelsFromMediaSource returns the labels from the underlying driver in the MediaSource.
func LabelsFromMediaSource[T, U any](src MediaSource[T]) ([]string, error) {
	d, err := DriverFromMediaSource[T, U](src)
	if err != nil {
		return nil, err
	}
	return strings.Split(d.Info().Label, camera.LabelSeparator), nil
}

// DriverFromMediaSource returns the underlying driver from the MediaSource.
func DriverFromMediaSource[T, U any](src MediaSource[T]) (driver.Driver, error) {
	if asMedia, ok := src.(*mediaSource[T, U]); ok {
		if asMedia.driver != nil {
			return asMedia.driver, nil
		}
	}
	return nil, errors.Errorf("cannot convert media source (type %T) to type (%T)", src, (*mediaSource[T, U])(nil))
}

// newMediaSource instantiates a new media read closer and possibly references the given driver.
func newMediaSource[T, U any](d driver.Driver, r MediaReader[T], p U) MediaSource[T] {
	if d != nil {
		driverRefs.mu.Lock()
		defer driverRefs.mu.Unlock()

		label := d.Info().Label
		if rcv, ok := driverRefs.refs[label]; ok {
			rcv.Ref()
		} else {
			driverRefs.refs[label] = utils.NewRefCountedValue(d)
			driverRefs.refs[label].Ref()
		}
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	ms := &mediaSource[T, U]{
		driver:            d,
		reader:            r,
		props:             p,
		rootCancelCtx:     cancelCtx,
		rootCancel:        cancel,
		producerConsumers: map[string]*producerConsumer[T, U]{},
	}
	return ms
}

func (pc *producerConsumer[T, U]) start() {
	var startLocalCtx context.Context
	var span *trace.Span

	func() {
		pc.cancelCtxMu.RLock()
		defer pc.cancelCtxMu.RUnlock()
		startLocalCtx, span = trace.StartSpan(pc.cancelCtx, "gostream::producerConsumer::start")
	}()

	pc.listenersMu.Lock()
	defer pc.listenersMu.Unlock()

	pc.listeners++

	if pc.listeners != 1 {
		span.End()
		return
	}

	pc.activeBackgroundWorkers.Add(1)

	utils.ManagedGo(func() {
		defer span.End()

		first := true
		for {
			pc.cancelCtxMu.RLock()
			if pc.cancelCtx.Err() != nil {
				pc.cancelCtxMu.RUnlock()
				return
			}
			pc.cancelCtxMu.RUnlock()

			waitForNext := func() (int64, bool) {
				_, waitForNextSpan := trace.StartSpan(startLocalCtx, "gostream::producerConsumer::waitForNext")
				defer waitForNextSpan.End()

				for {
					pc.producerCond.L.Lock()
					requests := atomic.LoadInt64(&pc.interestedConsumers)
					if requests == 0 {
						pc.cancelCtxMu.RLock()
						if err := pc.cancelCtx.Err(); err != nil {
							pc.cancelCtxMu.RUnlock()
							pc.producerCond.L.Unlock()
							return 0, false
						}
						pc.cancelCtxMu.RUnlock()

						pc.producerCond.Wait()
						requests = atomic.LoadInt64(&pc.interestedConsumers)
						pc.producerCond.L.Unlock()
						if requests == 0 {
							continue
						}
					} else {
						pc.producerCond.L.Unlock()
					}
					return requests, true
				}
			}
			requests, cont := waitForNext()
			if !cont {
				return
			}

			pc.cancelCtxMu.RLock()
			if err := pc.cancelCtx.Err(); err != nil {
				pc.cancelCtxMu.RUnlock()
				return
			}
			pc.cancelCtxMu.RUnlock()

			func() {
				var doReadSpan *trace.Span
				startLocalCtx, doReadSpan = trace.StartSpan(startLocalCtx, "gostream::producerConsumer (anonymous function to read)")

				defer func() {
					pc.producerCond.L.Lock()
					atomic.AddInt64(&pc.interestedConsumers, -requests)
					pc.consumerCond.Broadcast()
					pc.producerCond.L.Unlock()
					doReadSpan.End()
				}()

				var lastRelease func()
				if !first {
					// okay to not hold a lock because we are the only both reader AND writer;
					// other goroutines are just readers.
					lastRelease = pc.current.Release
				} else {
					first = false
				}

				startLocalCtx, span := trace.StartSpan(startLocalCtx, "gostream::producerConsumer::readWrapper::Read")
				media, release, err := pc.readWrapper.Read(startLocalCtx)
				span.End()

				ref := utils.NewRefCountedValue(struct{}{})
				ref.Ref()

				// hold write lock long enough to set current but not for lastRelease
				// since the reader (who will call ref) will hold a similar read lock
				// to ref before unlocking. This ordering makes sure that we only ever
				// call a deref of the previous media once a new one can be fetched.
				pc.currentMu.Lock()
				pc.current = &mediaRefReleasePairWithError[T]{media, ref, func() {
					if ref.Deref() {
						if release != nil {
							release()
						}
					}
				}, err}
				pc.currentMu.Unlock()
				if lastRelease != nil {
					lastRelease()
				}
			}()
		}
	}, func() { defer pc.activeBackgroundWorkers.Done(); pc.cancel() })
}

type mediaRefReleasePairWithError[T any] struct {
	Media   T
	Ref     utils.RefCountedValue
	Release func()
	Err     error
}

func (pc *producerConsumer[T, U]) Stop() {
	pc.stateMu.Lock()
	defer pc.stateMu.Unlock()

	pc.stop()
}

// assumes stateMu lock is held.
func (pc *producerConsumer[T, U]) stop() {
	var span *trace.Span
	func() {
		pc.cancelCtxMu.RLock()
		defer pc.cancelCtxMu.RUnlock()
		_, span = trace.StartSpan(pc.cancelCtx, "gostream::producerConsumer::stop")
	}()

	defer span.End()

	pc.cancel()

	pc.producerCond.L.Lock()
	pc.producerCond.Signal()
	pc.producerCond.L.Unlock()
	pc.consumerCond.L.Lock()
	pc.consumerCond.Broadcast()
	pc.consumerCond.L.Unlock()
	pc.activeBackgroundWorkers.Wait()

	// reset
	cancelCtx, cancel := context.WithCancel(WithMIMETypeHint(pc.rootCancelCtx, pc.mimeType))
	pc.cancelCtxMu.Lock()
	pc.cancelCtx = cancelCtx
	pc.cancelCtxMu.Unlock()
	pc.cancel = cancel
}

func (pc *producerConsumer[T, U]) stopOne() {
	var span *trace.Span
	func() {
		pc.cancelCtxMu.RLock()
		defer pc.cancelCtxMu.RUnlock()
		_, span = trace.StartSpan(pc.cancelCtx, "gostream::producerConsumer::stopOne")
	}()

	defer span.End()

	pc.stateMu.Lock()
	defer pc.stateMu.Unlock()
	pc.listenersMu.Lock()
	defer pc.listenersMu.Unlock()
	pc.listeners--
	if pc.listeners == 0 {
		pc.stop()
	}
}

func (ms *mediaSource[T, U]) MediaProperties(_ context.Context) (U, error) {
	return ms.props, nil
}

// MediaReleasePairWithError contains the result of fetching media.
type MediaReleasePairWithError[T any] struct {
	Media   T
	Release func()
	Err     error
}

// NewMediaStreamForChannel returns a MediaStream backed by a channel.
func NewMediaStreamForChannel[T any](ctx context.Context) (context.Context, MediaStream[T], chan<- MediaReleasePairWithError[T]) {
	cancelCtx, cancel := context.WithCancel(ctx)
	ch := make(chan MediaReleasePairWithError[T])
	return cancelCtx, &mediaStreamFromChannel[T]{
		media:     ch,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}, ch
}

type mediaStreamFromChannel[T any] struct {
	media     chan MediaReleasePairWithError[T]
	cancelCtx context.Context
	cancel    func()
}

func (ms *mediaStreamFromChannel[T]) Next(ctx context.Context) (T, func(), error) {
	ctx, span := trace.StartSpan(ctx, "gostream::mediaStreamFromChannel::Next")
	defer span.End()

	var zero T
	select {
	case <-ms.cancelCtx.Done():
		return zero, nil, ms.cancelCtx.Err()
	case <-ctx.Done():
		return zero, nil, ctx.Err()
	case pair := <-ms.media:
		return pair.Media, pair.Release, pair.Err
	}
}

func (ms *mediaStreamFromChannel[T]) Close(ctx context.Context) error {
	ms.cancel()
	return nil
}

type mediaStream[T any, U any] struct {
	mu        sync.Mutex
	ms        *mediaSource[T, U]
	prodCon   *producerConsumer[T, U]
	cancelCtx context.Context
	cancel    func()
}

func (ms *mediaStream[T, U]) Next(ctx context.Context) (T, func(), error) {
	ctx, nextSpan := trace.StartSpan(ctx, "gostream::mediaStream::Next")
	defer nextSpan.End()

	ms.mu.Lock()
	defer ms.mu.Unlock()
	// lock keeps us sequential and prevents misuse

	var zero T
	if err := ms.cancelCtx.Err(); err != nil {
		return zero, nil, err
	}

	ms.prodCon.consumerCond.L.Lock()
	// Even though interestedConsumers is atomic, this is a critical section!
	// That's because if the producer sees zero interested consumers, it's going
	// to Wait but we only want it to do that once we are ready to signal it.
	// It's also a RLock because we have many consumers (readers) and one producer (writer).
	atomic.AddInt64(&ms.prodCon.interestedConsumers, 1)
	ms.prodCon.producerCond.Signal()

	select {
	case <-ms.cancelCtx.Done():
		ms.prodCon.consumerCond.L.Unlock()
		return zero, nil, ms.cancelCtx.Err()
	case <-ctx.Done():
		ms.prodCon.consumerCond.L.Unlock()
		return zero, nil, ctx.Err()
	default:
	}

	waitForNext := func() error {
		_, span := trace.StartSpan(ctx, "gostream::mediaStream::Next::waitForNext")
		defer span.End()

		ms.prodCon.consumerCond.Wait()
		ms.prodCon.consumerCond.L.Unlock()
		if err := ms.cancelCtx.Err(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}

	if err := waitForNext(); err != nil {
		return zero, nil, err
	}

	isAvailable := func() bool {
		ms.prodCon.currentMu.RLock()
		available := ms.prodCon.current != nil
		ms.prodCon.currentMu.RUnlock()
		return available
	}
	for !isAvailable() {
		ms.prodCon.consumerCond.L.Lock()
		if err := waitForNext(); err != nil {
			return zero, nil, err
		}
	}

	ctx, prodConLockSpan := trace.StartSpan(ctx, "gostream::mediaStream::Next (waiting for ms.prodCon lock)")
	defer prodConLockSpan.End()

	// hold a read lock long enough before current.Ref can be dereffed
	// due to a new current being set.
	ms.prodCon.currentMu.RLock()
	defer ms.prodCon.currentMu.RUnlock()
	current := ms.prodCon.current
	if current.Err != nil {
		return zero, nil, current.Err
	}
	current.Ref.Ref()
	return current.Media, current.Release, nil
}

func (ms *mediaStream[T, U]) Close(ctx context.Context) error {
	if parentSpan := trace.FromContext(ctx); parentSpan != nil {
		func() {
			ms.prodCon.cancelCtxMu.Lock()
			defer ms.prodCon.cancelCtxMu.Unlock()
			ms.prodCon.cancelCtx = trace.NewContext(ms.prodCon.cancelCtx, parentSpan)
		}()
	}

	var span *trace.Span
	func() {
		ms.prodCon.cancelCtxMu.Lock()
		defer ms.prodCon.cancelCtxMu.Unlock()
		ms.prodCon.cancelCtx, span = trace.StartSpan(ms.prodCon.cancelCtx, "gostream::mediaStream::Close")
	}()

	defer span.End()

	ms.cancel()
	ms.prodCon.errHandlersMu.Lock()
	delete(ms.prodCon.errHandlers, ms)
	ms.prodCon.errHandlersMu.Unlock()
	ms.prodCon.stopOne()
	return nil
}

func (ms *mediaSource[T, U]) Stream(ctx context.Context, errHandlers ...ErrorHandler) (MediaStream[T], error) {
	ms.producerConsumersMu.Lock()
	mimeType := MIMETypeHint(ctx, "")
	prodCon, ok := ms.producerConsumers[mimeType]
	if !ok {
		// TODO(erd): better to have no max like this and instead clean up over time.
		if len(ms.producerConsumers)+1 == 256 {
			return nil, errors.New("reached max producer consumers of 256")
		}
		cancelCtx, cancel := context.WithCancel(WithMIMETypeHint(ms.rootCancelCtx, mimeType))
		condMu := &sync.RWMutex{}
		producerCond := sync.NewCond(condMu)
		consumerCond := sync.NewCond(condMu.RLocker())

		prodCon = &producerConsumer[T, U]{
			rootCancelCtx: ms.rootCancelCtx,
			cancelCtx:     cancelCtx,
			cancelCtxMu:   &sync.RWMutex{},
			cancel:        cancel,
			mimeType:      mimeType,
			producerCond:  producerCond,
			consumerCond:  consumerCond,
			condMu:        condMu,
			errHandlers:   map[*mediaStream[T, U]][]ErrorHandler{},
		}
		prodCon.readWrapper = MediaReaderFunc[T](func(ctx context.Context) (T, func(), error) {
			media, release, err := ms.reader.Read(ctx)
			if err == nil {
				return media, release, nil
			}

			prodCon.errHandlersMu.Lock()
			defer prodCon.errHandlersMu.Unlock()
			for _, handlers := range prodCon.errHandlers {
				for _, handler := range handlers {
					handler(ctx, err)
				}
			}
			var zero T
			return zero, nil, err
		})
		ms.producerConsumers[mimeType] = prodCon
	}
	ms.producerConsumersMu.Unlock()

	prodCon.stateMu.Lock()
	defer prodCon.stateMu.Unlock()

	if currentSpan := trace.FromContext(ctx); currentSpan != nil {
		func() {
			prodCon.cancelCtxMu.Lock()
			defer prodCon.cancelCtxMu.Unlock()
			prodCon.cancelCtx = trace.NewContext(prodCon.cancelCtx, currentSpan)
		}()
	}

	var cancelCtx context.Context
	var cancel context.CancelFunc
	func() {
		prodCon.cancelCtxMu.RLock()
		defer prodCon.cancelCtxMu.RUnlock()
		cancelCtx, cancel = context.WithCancel(prodCon.cancelCtx)
	}()
	stream := &mediaStream[T, U]{
		ms:        ms,
		prodCon:   prodCon,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}

	if len(errHandlers) != 0 {
		prodCon.errHandlersMu.Lock()
		prodCon.errHandlers[stream] = errHandlers
		prodCon.errHandlersMu.Unlock()
	}
	prodCon.start()

	return stream, nil
}

func (ms *mediaSource[T, U]) Close(ctx context.Context) error {
	func() {
		ms.producerConsumersMu.Lock()
		defer ms.producerConsumersMu.Unlock()
		for _, prodCon := range ms.producerConsumers {
			prodCon.Stop()
		}
	}()
	err := ms.reader.Close(ctx)

	if ms.driver == nil {
		return err
	}
	driverRefs.mu.Lock()
	defer driverRefs.mu.Unlock()

	label := ms.driver.Info().Label
	if rcv, ok := driverRefs.refs[label]; ok {
		if rcv.Deref() {
			delete(driverRefs.refs, label)
			return multierr.Combine(err, ms.driver.Close())
		}
	} else {
		return multierr.Combine(err, ms.driver.Close())
	}

	// Do not close if a driver is being referenced. Client will decide what to do if
	// they encounter this error.
	return multierr.Combine(err, &DriverInUseError{label})
}
