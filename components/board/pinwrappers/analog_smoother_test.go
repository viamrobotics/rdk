package pinwrappers

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
)

type testAnalog struct {
	mu   sync.Mutex
	r    *rand.Rand
	n    int64
	lim  int64
	stop bool
}

func (t *testAnalog) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stop || t.n >= t.lim {
		return 0, errStopReading
	}
	t.n++
	return t.r.Intn(100), nil
}

func (t *testAnalog) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	return grpc.UnimplementedError
}

func (t *testAnalog) Close(ctx context.Context) error {
	return nil
}

func TestAnalogSmoother1(t *testing.T) {
	testReader := testAnalog{
		r:   rand.New(rand.NewSource(11)),
		lim: 200,
	}
	defer func() {
		testReader.mu.Lock()
		defer testReader.mu.Unlock()
		testReader.stop = true
	}()

	logger := logging.NewTestLogger(t)
	as := SmoothAnalogReader(&testReader, board.AnalogReaderConfig{
		AverageOverMillis: 10,
		SamplesPerSecond:  10000,
	}, logger)

	testutils.WaitForAssertionWithSleep(t, 10*time.Millisecond, 200, func(tb testing.TB) {
		tb.Helper()
		v, err := as.Read(context.Background(), nil)
		test.That(tb, err, test.ShouldEqual, errStopReading)
		test.That(tb, v, test.ShouldEqual, 52)

		// need lock to access testReader.n
		testReader.mu.Lock()
		defer testReader.mu.Unlock()
		test.That(tb, testReader.n, test.ShouldEqual, testReader.lim)
	})

	test.That(t, as.Close(context.Background()), test.ShouldBeNil)
}
