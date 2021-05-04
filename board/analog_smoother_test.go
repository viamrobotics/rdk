package board

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/robotcore/utils"
)

type testReader struct {
	mu   sync.Mutex
	r    *rand.Rand
	stop bool
}

func (t *testReader) Read(ctx context.Context) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stop {
		return 0, ErrStopReading
	}

	return t.r.Intn(100), nil
}

func TestAnalogSmoother1(t *testing.T) {

	testReader := testReader{
		r: rand.New(rand.NewSource(11)),
	}
	defer func() {
		testReader.mu.Lock()
		defer testReader.mu.Unlock()
		testReader.stop = true
	}()

	logger := golog.NewTestLogger(t)
	tmp := AnalogSmootherWrap(&testReader, AnalogConfig{}, logger)
	_, ok := tmp.(*AnalogSmoother)
	test.That(t, ok, test.ShouldBeFalse)

	as := AnalogSmootherWrap(&testReader, AnalogConfig{
		AverageOverMillis: 10,
		SamplesPerSecond:  10000,
	}, logger)
	_, ok = as.(*AnalogSmoother)
	test.That(t, ok, test.ShouldBeTrue)

	time.Sleep(200 * time.Millisecond)

	v, err := as.Read(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, v, test.ShouldAlmostEqual, 50.0, 10.0)

	err = utils.TryClose(as)
	test.That(t, err, test.ShouldBeNil)
}
