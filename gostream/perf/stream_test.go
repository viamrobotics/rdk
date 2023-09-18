package perf

import (
	"context"
	"flag"
	"image"
	"testing"
	"time"

	"go.viam.com/test"
	"golang.org/x/time/rate"

	"go.viam.com/rdk/gostream"
)

func init() {
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

func stream(ctx context.Context, b *testing.B, s gostream.VideoStream) {
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
	s := gostream.NewEmbeddedVideoStreamFromReader(r)

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
	s := gostream.NewEmbeddedVideoStreamFromReader(r)

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
	s := gostream.NewEmbeddedVideoStreamFromReader(r)
	ctx, cancel := context.WithCancel(context.Background())

	go stream(ctx, b, gostream.NewEmbeddedVideoStreamFromReader(r))

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
	s := gostream.NewEmbeddedVideoStreamFromReader(r)
	ctx, cancel := context.WithCancel(context.Background())

	go stream(ctx, b, gostream.NewEmbeddedVideoStreamFromReader(r))
	go stream(ctx, b, gostream.NewEmbeddedVideoStreamFromReader(r))

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
