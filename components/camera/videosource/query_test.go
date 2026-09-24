package videosource

import (
	"testing"

	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// panickingAdapter mimics a blackjack/webcam device that reports a stepwise frame interval.
type panickingAdapter struct {
	closed bool
}

func (a *panickingAdapter) Open() error  { return nil }
func (a *panickingAdapter) Close() error { a.closed = true; return nil }

func (a *panickingAdapter) Properties() []prop.Media {
	panic("reflect: reflect.Value.SetUint using value obtained using unexported field")
}

func (a *panickingAdapter) VideoRecord(prop.Media) (video.Reader, error) { return nil, nil }

// healthyAdapter is a device whose properties query succeeds.
type healthyAdapter struct{}

func (a *healthyAdapter) Open() error  { return nil }
func (a *healthyAdapter) Close() error { return nil }

func (a *healthyAdapter) Properties() []prop.Media {
	return []prop.Media{{Video: prop.Video{Width: 640, Height: 480}}}
}

func (a *healthyAdapter) VideoRecord(prop.Media) (video.Reader, error) { return nil, nil }

func TestQueryDriverPropertiesRecoversPanic(t *testing.T) {
	const (
		badLabel  = "test-panicking-driver"
		goodLabel = "test-healthy-driver"
	)
	bad := &panickingAdapter{}
	manager := driver.GetManager()
	test.That(t, manager.Register(bad, driver.Info{Label: badLabel, DeviceType: driver.Camera}), test.ShouldBeNil)
	test.That(t, manager.Register(&healthyAdapter{}, driver.Info{Label: goodLabel, DeviceType: driver.Camera}), test.ShouldBeNil)
	t.Cleanup(func() {
		for _, d := range manager.Query(driver.FilterFn(func(d driver.Driver) bool {
			return d.Info().Label == badLabel || d.Info().Label == goodLabel
		})) {
			manager.Delete(d.ID())
		}
	})

	filter := driver.FilterAnd(getVideoFilterBase(), driver.FilterFn(func(d driver.Driver) bool {
		return d.Info().Label == badLabel || d.Info().Label == goodLabel
	}))
	m := queryDriverProperties(filter, logging.NewTestLogger(t))

	test.That(t, m, test.ShouldHaveLength, 1)
	for d, props := range m {
		test.That(t, d.Info().Label, test.ShouldEqual, goodLabel)
		test.That(t, props, test.ShouldHaveLength, 1)
	}
	// The panicking driver was opened for the query, so it must still be closed.
	test.That(t, bad.closed, test.ShouldBeTrue)
}
