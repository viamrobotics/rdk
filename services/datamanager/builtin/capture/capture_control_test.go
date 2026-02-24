package capture

import (
	"context"
	"testing"

	"github.com/benbjohnson/clock"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
)

func float32Ptr(f float32) *float32 { return &f }

// mockCollector implements data.Collector for testing without real capture infrastructure.
type mockCollector struct {
	closed bool
}

func (m *mockCollector) Close()   { m.closed = true }
func (m *mockCollector) Flush()   {}
func (m *mockCollector) Collect() {}

func TestCaptureConfigsEqual(t *testing.T) {
	for _, tc := range []struct {
		name     string
		a        map[string]datamanager.CaptureConfigReading
		b        map[string]datamanager.CaptureConfigReading
		expected bool
	}{
		{
			name:     "nil maps are equal",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "nil and empty map are equal",
			a:        nil,
			b:        map[string]datamanager.CaptureConfigReading{},
			expected: true,
		},
		{
			name: "equal configs",
			a: map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {
				CaptureFrequencyHz: float32Ptr(10.0), Tags: []string{"tag1"},
			}},
			b: map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {
				CaptureFrequencyHz: float32Ptr(10.0), Tags: []string{"tag1"},
			}},
			expected: true,
		},
		{
			name: "different frequency",
			a: map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {
				CaptureFrequencyHz: float32Ptr(10.0),
			}},
			b: map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {
				CaptureFrequencyHz: float32Ptr(5.0),
			}},
			expected: false,
		},
		{
			name:     "nil freq vs non-nil freq",
			a:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {CaptureFrequencyHz: nil}},
			b:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(10.0)}},
			expected: false,
		},
		{
			name:     "nil tags vs non-nil tags",
			a:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {Tags: nil}},
			b:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {Tags: []string{"tag1"}}},
			expected: false,
		},
		{
			name:     "nil tags vs empty tags are different",
			a:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {Tags: nil}},
			b:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {Tags: []string{}}},
			expected: false,
		},
		{
			name:     "different keys",
			a:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {}},
			b:        map[string]datamanager.CaptureConfigReading{"camera-2/GetImages": {}},
			expected: false,
		},
		{
			name:     "different lengths",
			a:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {}, "camera-2/GetImages": {}},
			b:        map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {}},
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, captureConfigsEqual(tc.a, tc.b), test.ShouldEqual, tc.expected)
		})
	}
}

// newTestCapture returns a Capture with the given baseCollectorConfigs and
// an optional pre-populated collectors map. captureDir is always a temp dir.
func newTestCapture(
	t *testing.T,
	baseCollectorConfigs CollectorConfigsByResource,
	existingCollectors collectors,
	currentCaptureConfig map[string]datamanager.CaptureConfigReading,
) *Capture {
	t.Helper()
	if existingCollectors == nil {
		existingCollectors = make(collectors)
	}
	return &Capture{
		logger:               logging.NewTestLogger(t),
		clk:                  clock.New(),
		collectors:           existingCollectors,
		captureDir:           t.TempDir(),
		maxCaptureFileSize:   256 * 1024,
		baseCollectorConfigs: baseCollectorConfigs,
		currentCaptureConfig: currentCaptureConfig,
	}
}

func TestSetCaptureConfig(t *testing.T) {
	armCfg := datamanager.DataCaptureConfig{
		Name:               arm.Named("arm-1"),
		Method:             "EndPosition",
		CaptureFrequencyHz: 1.0,
	}
	armMD := newCollectorMetadata(armCfg)
	mock1 := &mockCollector{} // used by "disables collector" case
	mock2 := &mockCollector{} // used by "service-level tags" case

	for _, tc := range []struct {
		name          string
		baseConfigs   CollectorConfigsByResource
		existingColls collectors
		currentConfig map[string]datamanager.CaptureConfigReading
		baseTags      []string
		input         map[string]datamanager.CaptureConfigReading
		verify        func(t *testing.T, c *Capture)
	}{
		{
			name: "no-op when configs are equal",
			currentConfig: map[string]datamanager.CaptureConfigReading{
				"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(10.0)},
			},
			input: map[string]datamanager.CaptureConfigReading{
				"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(10.0)},
			},
			verify: func(t *testing.T, c *Capture) {
				test.That(t, c.currentCaptureConfig, test.ShouldResemble, map[string]datamanager.CaptureConfigReading{
					"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(10.0)},
				})
			},
		},
		{
			name:          "disables collector on zero frequency",
			baseConfigs:   CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{armCfg}},
			existingColls: collectors{armMD: {Collector: mock1, Config: armCfg}},
			currentConfig: map[string]datamanager.CaptureConfigReading{
				"arm-1/EndPosition": {CaptureFrequencyHz: float32Ptr(1.0)},
			},
			input: map[string]datamanager.CaptureConfigReading{
				"arm-1/EndPosition": {CaptureFrequencyHz: float32Ptr(0)},
			},
			verify: func(t *testing.T, c *Capture) {
				test.That(t, mock1.closed, test.ShouldBeTrue)
				c.collectorsMu.Lock()
				_, exists := c.collectors[armMD]
				c.collectorsMu.Unlock()
				test.That(t, exists, test.ShouldBeFalse)
			},
		},
		{
			name: "reverts to base config on nil input",
			baseConfigs: CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{
				{Name: arm.Named("arm-1"), Method: "EndPosition", CaptureFrequencyHz: 1.0, Disabled: true},
			}},
			currentConfig: map[string]datamanager.CaptureConfigReading{
				"arm-1/EndPosition": {CaptureFrequencyHz: float32Ptr(5.0)},
			},
			input: nil,
			verify: func(t *testing.T, c *Capture) {
				test.That(t, c.currentCaptureConfig, test.ShouldBeNil)
				c.collectorsMu.Lock()
				test.That(t, len(c.collectors), test.ShouldEqual, 0)
				c.collectorsMu.Unlock()
			},
		},
		{
			name:          "service-level tags are overridden by capture config tags",
			baseConfigs:   CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{armCfg}},
			existingColls: collectors{armMD: {Collector: mock2, Config: armCfg}},
			baseTags:      []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"arm-1/EndPosition": {Tags: []string{"override-tag"}},
			},
			verify: func(t *testing.T, c *Capture) {
				test.That(t, c.currentCaptureConfig["arm-1/EndPosition"].Tags, test.ShouldResemble, []string{"override-tag"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCapture(t, tc.baseConfigs, tc.existingColls, tc.currentConfig)
			c.baseTags = tc.baseTags
			c.SetCaptureConfigs(context.Background(), tc.input)
			tc.verify(t, c)
		})
	}
}
