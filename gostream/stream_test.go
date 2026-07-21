package gostream

import (
	"context"
	"flag"
	"image"
	"testing"
	"time"

	"go.viam.com/test"
	"golang.org/x/time/rate"

	"go.viam.com/rdk/gostream/codec"
	"go.viam.com/rdk/logging"
)

func init() {
	testing.Init()
	_ = flag.Set("test.benchtime", "100x")
}

type reader struct {
	img     image.Image
	limiter *rate.Limiter
}

func newReader(fps float64) *reader {
	limiter := rate.NewLimiter(rate.Limit(fps), 1)
	return &reader{
		img:     image.Image(image.Rect(0, 0, 0, 0)),
		limiter: limiter,
	}
}

func (r *reader) Close(_ context.Context) error { return nil }
func (r *reader) Read(_ context.Context) (image.Image, func(), error) {
	_ = r.limiter.Wait(context.Background())
	return r.img, func() {}, nil
}

func stream(ctx context.Context, b *testing.B, s VideoStream) {
	b.Helper()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, _, err := s.Next(context.Background())
			test.That(b, err, test.ShouldBeNil)
		}
	}
}

const SecondNs = 1000000000.0 // second in nanoseconds

func incrementAverage(avgOld, valNew, sizeNew float64) float64 {
	avgNew := (avgOld) + (valNew-avgOld)/sizeNew
	return avgNew
}

func BenchmarkStream_30FPS(b *testing.B) {
	r := newReader(30)
	s := NewEmbeddedVideoStreamFromReader(r)

	var avgNs float64
	var count int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		b.StartTimer()
		_, _, err := s.Next(context.Background())
		b.StopTimer()

		test.That(b, err, test.ShouldBeNil)
		count++
		elapsedNs := time.Since(start).Nanoseconds()
		avgNs += (float64(elapsedNs) - avgNs) / float64(count)
		avgNs = incrementAverage(avgNs, float64(elapsedNs), float64(count))
	}

	b.ReportMetric(SecondNs/avgNs, "fps")
}

func BenchmarkStream_60FPS(b *testing.B) {
	r := newReader(60)
	s := NewEmbeddedVideoStreamFromReader(r)

	var avgNs float64
	var count int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		b.StartTimer()
		_, _, err := s.Next(context.Background())
		b.StopTimer()

		test.That(b, err, test.ShouldBeNil)
		count++
		elapsedNs := time.Since(start).Nanoseconds()
		avgNs += (float64(elapsedNs) - avgNs) / float64(count)
		avgNs = incrementAverage(avgNs, float64(elapsedNs), float64(count))
	}

	b.ReportMetric(SecondNs/avgNs, "fps")
}

func BenchmarkStream_30FPS_2Streams(b *testing.B) {
	r := newReader(30)
	s := NewEmbeddedVideoStreamFromReader(r)
	ctx, cancel := context.WithCancel(context.Background())

	go stream(ctx, b, NewEmbeddedVideoStreamFromReader(r))

	var avgNs float64
	var count int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		b.StartTimer()
		_, _, err := s.Next(context.Background())
		b.StopTimer()

		test.That(b, err, test.ShouldBeNil)
		count++
		elapsedNs := time.Since(start).Nanoseconds()
		avgNs += (float64(elapsedNs) - avgNs) / float64(count)
		avgNs = incrementAverage(avgNs, float64(elapsedNs), float64(count))
	}

	cancel()
	b.ReportMetric(SecondNs/avgNs, "fps")
}

func BenchmarkStream_30FPS_3Streams(b *testing.B) {
	r := newReader(30)
	s := NewEmbeddedVideoStreamFromReader(r)
	ctx, cancel := context.WithCancel(context.Background())

	go stream(ctx, b, NewEmbeddedVideoStreamFromReader(r))
	go stream(ctx, b, NewEmbeddedVideoStreamFromReader(r))

	var avgNs float64
	var count int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		b.StartTimer()
		_, _, err := s.Next(context.Background())
		b.StopTimer()

		test.That(b, err, test.ShouldBeNil)
		count++
		elapsedNs := time.Since(start).Nanoseconds()
		avgNs += (float64(elapsedNs) - avgNs) / float64(count)
		avgNs = incrementAverage(avgNs, float64(elapsedNs), float64(count))
	}

	cancel()
	b.ReportMetric(SecondNs/avgNs, "fps")
}

type fakeEncoder struct{}

func (f *fakeEncoder) Encode(_ context.Context, _ image.Image) ([]byte, error) { return nil, nil }
func (f *fakeEncoder) Close() error                                            { return nil }

type fakeEncoderFactory struct{}

func (f *fakeEncoderFactory) New(_, _, _ int, _ logging.Logger) (codec.VideoEncoder, error) {
	return &fakeEncoder{}, nil
}
func (f *fakeEncoderFactory) MIMEType() string { return "test/fake" }

func TestNewStream_TargetFrameRateGuard(t *testing.T) {
	logger := logging.NewTestLogger(t)
	factory := &fakeEncoderFactory{}

	tests := []struct {
		name       string
		input      int
		wantResult int
	}{
		{"unset (zero)", 0, defaultTargetFrameRate},
		{"negative", -5, defaultTargetFrameRate},
		{"in range", 60, 60},
		{"at max", maxTargetFrameRate, maxTargetFrameRate},
		{"above max", maxTargetFrameRate + 1, defaultTargetFrameRate},
		{"garbage huge", 1_000_000_000, defaultTargetFrameRate},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewStream(StreamConfig{
				VideoEncoderFactory: factory,
				TargetFrameRate:     tc.input,
			}, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, s.(*basicStream).config.TargetFrameRate, test.ShouldEqual, tc.wantResult)
		})
	}
}
