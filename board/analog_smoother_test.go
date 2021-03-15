package board

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testReader struct {
	r    *rand.Rand
	stop bool
}

func (t *testReader) Read() (int, error) {
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

	as := AnalogSmoother{
		Raw:               &testReader,
		AverageOverMillis: 10,
		SamplesPerSecond:  10000,
	}
	as.Start()

	assert.Equal(t, 100, as.data.NumSamples())

	time.Sleep(200 * time.Millisecond)

	v, err := as.Read()
	if err != nil {
		t.Fatal(err)
	}
	assert.InDelta(t, 50.0, v, 1.0)
}
