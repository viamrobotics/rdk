package capture

import (
	"context"
	"sync"
	"testing"

	"github.com/benbjohnson/clock"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
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

// cameraAPI is constructed without importing the camera package to avoid triggering its
// init(), which would conflict with our mock collector registration below.
var cameraAPI = resource.APINamespaceRDK.WithComponentType("camera")

// registerCameraCollector registers a no-op collector constructor for camera/GetImages so that
// tests which trigger collector rebuilds don't fail on a missing constructor lookup.
var registerCameraCollectorOnce sync.Once

func registerCameraCollector() {
	registerCameraCollectorOnce.Do(func() {
		data.RegisterCollector(
			data.MethodMetadata{API: cameraAPI, MethodName: "GetImages"},
			func(_ interface{}, _ data.CollectorParams) (data.Collector, error) {
				return &mockCollector{}, nil
			},
		)
	})
}

// newTestCapture returns a Capture with the given baseCollectorConfigs and
// an optional pre-populated collectors map. captureDir is always a temp dir.
func newTestCapture(
	t *testing.T,
	baseCollectorConfigs CollectorConfigsByResource,
	existingCollectors collectors,
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
	}
}

func TestSetCaptureConfig(t *testing.T) {
	cameraCfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(cameraAPI, "camera-1"),
		Method:             "GetImages",
		CaptureFrequencyHz: 1.0,
	}
	cameraMD := newCollectorMetadata(cameraCfg)
	mock1 := &mockCollector{} // used by "disables collector" case
	mock2 := &mockCollector{} // used by "service-level tags" case
	mock3 := &mockCollector{} // used by "no-op" case

	for _, tc := range []struct {
		name                   string
		baseConfigs            CollectorConfigsByResource
		existingColls          collectors
		baseTags               []string
		input                  map[string]datamanager.CaptureConfigReading
		expectedClosed         *mockCollector
		expectedNotClosed      *mockCollector
		expectedCollectorCount int
		expectedNewTags        []string
	}{
		{
			name:                   "no-op when effective config is unchanged",
			baseConfigs:            CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{cameraCfg}},
			existingColls:          collectors{cameraMD: {Collector: mock3, Config: cameraCfg}},
			input:                  map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(1.0)}},
			expectedNotClosed:      mock3,
			expectedCollectorCount: 1,
		},
		{
			name:                   "disables collector on zero frequency",
			baseConfigs:            CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{cameraCfg}},
			existingColls:          collectors{cameraMD: {Collector: mock1, Config: cameraCfg}},
			input:                  map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {CaptureFrequencyHz: float32Ptr(0)}},
			expectedClosed:         mock1,
			expectedCollectorCount: 0,
		},
		{
			name: "reverts to base config on nil input",
			baseConfigs: CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{
				{Name: resource.NewName(cameraAPI, "camera-1"), Method: "GetImages", CaptureFrequencyHz: 1.0, Disabled: true},
			}},
			input:                  nil,
			expectedCollectorCount: 0,
		},
		{
			name:                   "service-level tags are overridden by capture config tags",
			baseConfigs:            CollectorConfigsByResource{nil: []datamanager.DataCaptureConfig{cameraCfg}},
			existingColls:          collectors{cameraMD: {Collector: mock2, Config: cameraCfg}},
			baseTags:               []string{"service-tag"},
			input:                  map[string]datamanager.CaptureConfigReading{"camera-1/GetImages": {Tags: []string{"override-tag"}}},
			expectedClosed:         mock2,
			expectedCollectorCount: 1,
			expectedNewTags:        []string{"override-tag"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registerCameraCollector()
			c := newTestCapture(t, tc.baseConfigs, tc.existingColls)
			c.baseTags = tc.baseTags
			c.SetCaptureConfigs(context.Background(), tc.input)

			if tc.expectedClosed != nil {
				test.That(t, tc.expectedClosed.closed, test.ShouldBeTrue)
			}
			if tc.expectedNotClosed != nil {
				test.That(t, tc.expectedNotClosed.closed, test.ShouldBeFalse)
			}
			test.That(t, len(c.collectors), test.ShouldEqual, tc.expectedCollectorCount)
			if tc.expectedNewTags != nil {
				test.That(t, c.collectors[cameraMD].Config.Tags, test.ShouldResemble, tc.expectedNewTags)
			}
		})
	}
}
