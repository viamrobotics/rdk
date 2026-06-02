package capture

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/benbjohnson/clock"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

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

var (
	fakeAPI                   = resource.APINamespaceRDK.WithComponentType("fake")
	fakeRes resource.Resource = newFakeRes("fake-1")
)

// fakeResource is a minimal resource.Resource whose Name() uses fakeAPI. We need this (rather
// than inject.NewSensor) so the auto-enable code path — which derives a collector's API from
// res.Name().API — lines up with the fakeAPI/GetReadings constructor registered in
// registerFakeCollector.
type fakeResource struct {
	name resource.Name
}

func newFakeRes(name string) *fakeResource {
	return &fakeResource{name: resource.NewName(fakeAPI, name)}
}

func (f *fakeResource) Name() resource.Name { return f.name }
func (f *fakeResource) Reconfigure(_ context.Context, _ resource.Dependencies, _ resource.Config) error {
	return nil
}

func (f *fakeResource) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeResource) Status(_ context.Context) (map[string]interface{}, error) { return nil, nil }
func (f *fakeResource) Close(_ context.Context) error                            { return nil }

// registerFakeCollector registers no-op collector constructors so tests that trigger
// collector rebuilds don't fail on a missing constructor lookup. board/Analogs is included
// so the additional_params guard rail case actually exercises the additional_params check
// (not the upstream unknown-method check).
var registerFakeCollectorOnce sync.Once

func registerFakeCollector() {
	registerFakeCollectorOnce.Do(func() {
		data.RegisterCollector(
			data.MethodMetadata{API: fakeAPI, MethodName: "GetReadings"},
			func(_ interface{}, _ data.CollectorParams) (data.Collector, error) {
				return &mockCollector{}, nil
			},
		)
		boardAPI := resource.APINamespaceRDK.WithComponentType("board")
		data.RegisterCollector(
			data.MethodMetadata{API: boardAPI, MethodName: "Analogs"},
			func(_ interface{}, _ data.CollectorParams) (data.Collector, error) {
				return &mockCollector{}, nil
			},
		)
	})
}

// newTestCapture returns a Capture with the given defaultCollectorConfigs and
// an optional pre-populated collectors map. captureDir is always a temp dir.
// resourcesByShortName, when nil, defaults to an empty catalog.
func newTestCapture(
	t *testing.T,
	defaultCollectorConfigs CollectorConfigsByResource,
	existingCollectors collectors,
	resourcesByShortName map[string]resource.Resource,
	serviceTags []string,
) *Capture {
	t.Helper()
	if existingCollectors == nil {
		existingCollectors = make(collectors)
	}
	if resourcesByShortName == nil {
		resourcesByShortName = map[string]resource.Resource{}
	}
	return &Capture{
		logger:                  logging.NewTestLogger(t),
		clk:                     clock.New(),
		collectors:              existingCollectors,
		captureDir:              t.TempDir(),
		maxCaptureFileSize:      256 * 1024,
		defaultCollectorConfigs: defaultCollectorConfigs,
		resourcesByShortName:    resourcesByShortName,
		defaultTags:             serviceTags,
	}
}

func TestSetCaptureConfig(t *testing.T) {
	fakeCfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(fakeAPI, "fake-1"),
		Method:             "GetReadings",
		CaptureFrequencyHz: 1.0,
	}
	fakeCfgWithServiceTag := fakeCfg
	fakeCfgWithServiceTag.Tags = []string{"service-tag"}

	// A board-API resource so we can exercise the additional_params guard rail.
	boardAPI := resource.APINamespaceRDK.WithComponentType("board")
	boardRes := &fakeResource{name: resource.NewName(boardAPI, "board-1")}

	// fakeReading constructs a CaptureConfigReading with ResourceMethod populated for the
	// resource the test cases care about. Keeps the readings consistent across cases.
	fakeReading := func(name, method string, freq *float32, tags []string) datamanager.CaptureConfigReading {
		return datamanager.CaptureConfigReading{
			ResourceMethod:     datamanager.ResourceMethod{ResourceName: name, MethodName: method},
			CaptureFrequencyHz: freq,
			Tags:               tags,
		}
	}

	for _, tc := range []struct {
		name           string
		defaultConfigs CollectorConfigsByResource
		// existingCfg, when non-nil, pre-populates c.collectors with one collector built
		// from this config. A fresh *mockCollector is allocated per case so closed state
		// can't bleed between cases.
		existingCfg            *datamanager.DataCaptureConfig
		catalog                map[string]resource.Resource
		defaultTags            []string
		input                  map[string]datamanager.CaptureConfigReading
		expectExistingClosed   bool
		expectExistingOpen     bool
		expectedCollectorCount int
		// expectedTags, when non-nil, asserts every remaining collector has these tags.
		expectedTags []string
	}{
		// --- Static-config path: defaults present, sensor either matches or doesn't override. ---
		{
			name:           "no-op when effective config is unchanged",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg:    &fakeCfg,
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(1.0), nil),
			},
			expectExistingOpen:     true,
			expectedCollectorCount: 1,
		},
		{
			name:           "disables collector on zero frequency",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg:    &fakeCfg,
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(0), nil),
			},
			expectExistingClosed:   true,
			expectedCollectorCount: 0,
		},
		{
			name:           "disables collector on near-zero frequency",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg:    &fakeCfg,
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(1e-7), nil),
			},
			expectExistingClosed:   true,
			expectedCollectorCount: 0,
		},
		{
			// Existing collector was sensor-overridden to freq=5; sensor drops the override
			// (nil input). The override-derived collector should be closed and a fresh one
			// built at the default freq=1.
			name:           "reverts to default config on nil input",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg: &datamanager.DataCaptureConfig{
				Name: fakeCfg.Name, Method: fakeCfg.Method, CaptureFrequencyHz: 5.0,
			},
			input:                  nil,
			expectExistingClosed:   true,
			expectedCollectorCount: 1,
		},
		{
			name:           "service-level tags are overridden by capture config tags",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg:    &fakeCfg,
			defaultTags:    []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", nil, []string{"override-tag"}),
			},
			expectExistingClosed:   true,
			expectedCollectorCount: 1,
			expectedTags:           []string{"override-tag"},
		},
		{
			name:           "sensor-driven rebuild of static collector preserves service-level tags",
			defaultConfigs: CollectorConfigsByResource{fakeRes: {fakeCfg}},
			existingCfg:    &fakeCfgWithServiceTag,
			defaultTags:    []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(5.0), nil),
			},
			expectExistingClosed:   true,
			expectedCollectorCount: 1,
			expectedTags:           []string{"service-tag"},
		},

		// --- Auto-enable path: sensor names a resource/method pair not in defaultConfigs. ---
		{
			name:           "enables capture for a resource not in defaultConfigs",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"fake-1": fakeRes},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(1.0), nil),
			},
			expectedCollectorCount: 1,
		},
		{
			name:           "skips override for unknown resource",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{},
			input: map[string]datamanager.CaptureConfigReading{
				"unknown/GetReadings": fakeReading("unknown", "GetReadings", float32Ptr(1.0), nil),
			},
			expectedCollectorCount: 0,
		},
		{
			name:           "auto-enabled collector inherits service-level tags",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"fake-1": fakeRes},
			defaultTags:    []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": fakeReading("fake-1", "GetReadings", float32Ptr(1.0), nil),
			},
			expectedCollectorCount: 1,
			expectedTags:           []string{"service-tag"},
		},
		{
			name:           "skips auto-enable for method requiring additional_params",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"board-1": boardRes},
			input: map[string]datamanager.CaptureConfigReading{
				"board-1/Analogs": fakeReading("board-1", "Analogs", float32Ptr(1.0), nil),
			},
			expectedCollectorCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registerFakeCollector()

			var existing collectors
			var existingMock *mockCollector
			if tc.existingCfg != nil {
				existingMock = &mockCollector{}
				existing = collectors{
					newCollectorMetadata(*tc.existingCfg): {
						Resource: fakeRes, Collector: existingMock, Config: *tc.existingCfg,
					},
				}
			}

			c := newTestCapture(t, tc.defaultConfigs, existing, tc.catalog, tc.defaultTags)
			c.SetCaptureConfigs(tc.input)

			if tc.expectExistingClosed {
				test.That(t, existingMock, test.ShouldNotBeNil)
				test.That(t, existingMock.closed, test.ShouldBeTrue)
			}
			if tc.expectExistingOpen {
				test.That(t, existingMock, test.ShouldNotBeNil)
				test.That(t, existingMock.closed, test.ShouldBeFalse)
			}
			test.That(t, len(c.collectors), test.ShouldEqual, tc.expectedCollectorCount)

			if tc.expectedTags != nil {
				for _, cac := range c.collectors {
					test.That(t, cac.Config.Tags, test.ShouldResemble, tc.expectedTags)
				}
			}
		})
	}
}

func TestNearZeroFrequencySkipsCollector(t *testing.T) {
	registerFakeCollector()
	c := newTestCapture(t, nil, nil, nil, nil)

	fakeCfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(fakeAPI, "fake-1"),
		Method:             "GetReadings",
		CaptureFrequencyHz: 1e-7,
	}
	c.Reconfigure(context.Background(),
		nil,
		CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
		nil,
		Config{MaximumCaptureFileSizeBytes: 256 * 1024, CaptureDir: t.TempDir()},
	)

	test.That(t, len(c.collectors), test.ShouldEqual, 0)
}

// TestSensorEnabledCollectorClosedWhenOverrideDropped verifies that a collector enabled solely
// via the capture control sensor (no matching static config entry) is closed when the sensor
// stops returning that override on the next tick.
func TestSensorEnabledCollectorClosedWhenOverrideDropped(t *testing.T) {
	registerFakeCollector()
	c := newTestCapture(t, CollectorConfigsByResource{}, nil, map[string]resource.Resource{"fake-1": fakeRes}, nil)

	override := map[string]datamanager.CaptureConfigReading{
		"fake-1/GetReadings": {
			ResourceMethod:     datamanager.ResourceMethod{ResourceName: "fake-1", MethodName: "GetReadings"},
			CaptureFrequencyHz: float32Ptr(1.0),
		},
	}
	c.SetCaptureConfigs(override)
	test.That(t, len(c.collectors), test.ShouldEqual, 1)

	// Sensor stops returning the override; the orphan pass should close the collector.
	c.SetCaptureConfigs(nil)
	test.That(t, len(c.collectors), test.ShouldEqual, 0)
}

var (
	registerFileSizeCollectorOnce sync.Once
	fileSizeCollectorTarget       data.CaptureBufferedWriter
)

// TestMaxCaptureFileSize verifies that the MaximumCaptureFileSizeBytes config is correctly
// passed to the collector's capture buffer, and that changing it triggers a collector rebuild.
func TestMaxCaptureFileSize(t *testing.T) {
	const method = "GetFileSizeTestReadings"
	registerFileSizeCollectorOnce.Do(func() {
		data.RegisterCollector(
			data.MethodMetadata{API: fakeAPI, MethodName: method},
			func(_ interface{}, params data.CollectorParams) (data.Collector, error) {
				fileSizeCollectorTarget = params.Target
				return &mockCollector{}, nil
			},
		)
	})

	for _, tc := range []struct {
		name           string
		maxSizeChanges []int64 // MaximumCaptureFileSizeBytes for each successive Reconfigure call
		expectedFiles  int
	}{
		{
			name:           "configured max file size batches multiple readings into one file",
			maxSizeChanges: []int64{256 * 1024},
			expectedFiles:  1,
		},
		{
			name:           "changing max file size to 1 rebuilds collector and generates one file per reading",
			maxSizeChanges: []int64{256 * 1024, 1},
			expectedFiles:  3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captureDir := t.TempDir()
			fakeCfg := datamanager.DataCaptureConfig{
				Name:               resource.NewName(fakeAPI, "fake-1"),
				Method:             method,
				CaptureFrequencyHz: 1.0,
				CaptureDirectory:   captureDir,
			}
			c := &Capture{
				logger:     logging.NewTestLogger(t),
				clk:        clock.New(),
				collectors: make(collectors),
			}

			for _, maxSize := range tc.maxSizeChanges {
				c.Reconfigure(context.Background(),
					nil,
					CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
					nil,
					Config{MaximumCaptureFileSizeBytes: maxSize, CaptureDir: captureDir},
				)
			}

			tabularReading := &v1.SensorData{
				Metadata: &v1.SensorMetadata{},
				Data:     &v1.SensorData_Struct{Struct: &structpb.Struct{}},
			}
			for range 3 {
				test.That(t, fileSizeCollectorTarget.WriteTabular(tabularReading), test.ShouldBeNil)
			}

			entries, err := os.ReadDir(fileSizeCollectorTarget.Path())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(entries), test.ShouldEqual, tc.expectedFiles)
		})
	}
}
