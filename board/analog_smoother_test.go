package board

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

type testReader struct {
	r    *rand.Rand
	stop bool
}

func (t *testReader) Read(ctx context.Context) (int, error) {
	if t.stop {
		return 0, ErrStopReading
	}

	return t.r.Intn(100), nil
}

func TestAnalogSmoother1(t *testing.T) {

	testReader := testReader{
		rand.New(rand.NewSource(11)),
		false,
	}
	defer func() { testReader.stop = true }()

	logger := golog.NewTestLogger(t)
	tmp := AnalogSmootherWrap(context.Background(), &testReader, AnalogConfig{}, logger)
	_, ok := tmp.(*AnalogSmoother)
	assert.False(t, ok)

	as := AnalogSmootherWrap(context.Background(), &testReader, AnalogConfig{
		AverageOverMillis: 10,
		SamplesPerSecond:  10000,
	}, logger)
	_, ok = as.(*AnalogSmoother)
	assert.True(t, ok)

	time.Sleep(200 * time.Millisecond)

	v, err := as.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assert.InDelta(t, 50.0, v, 10.0)
}
