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

// registerFakeCollector registers a no-op collector constructor for fake/GetReadings so that
// tests which trigger collector rebuilds don't fail on a missing constructor lookup.
var registerFakeCollectorOnce sync.Once

func registerFakeCollector() {
	registerFakeCollectorOnce.Do(func() {
		data.RegisterCollector(
			data.MethodMetadata{API: fakeAPI, MethodName: "GetReadings"},
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
		serviceTags:             serviceTags,
	}
}

func TestSetCaptureConfig(t *testing.T) {
	fakeCfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(fakeAPI, "fake-1"),
		Method:             "GetReadings",
		CaptureFrequencyHz: 1.0,
	}
	fakeMD := newCollectorMetadata(fakeCfg)

	// A board-API resource so we can exercise the additional_params guard rail.
	boardAPI := resource.APINamespaceRDK.WithComponentType("board")
	boardRes := &fakeResource{name: resource.NewName(boardAPI, "board-1")}

	toggledCollector := &mockCollector{}
	tagsCollector := &mockCollector{}
	noopCollector := &mockCollector{}
	nearZeroCollector := &mockCollector{}

	for _, tc := range []struct {
		name                   string
		defaultConfigs         CollectorConfigsByResource
		existingColls          collectors
		catalog                map[string]resource.Resource
		defaultTags            []string
		input                  map[string]datamanager.CaptureConfigReading
		expectedClosed         *mockCollector
		expectedNotClosed      *mockCollector
		expectedCollectorCount int
		expectedNewTags        []string
	}{
		{
			name:                   "no-op when effective config is unchanged",
			defaultConfigs:         CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
			existingColls:          collectors{fakeMD: {Resource: fakeRes, Collector: noopCollector, Config: fakeCfg}},
			input:                  map[string]datamanager.CaptureConfigReading{"fake-1/GetReadings": {CaptureFrequencyHz: float32Ptr(1.0)}},
			expectedNotClosed:      noopCollector,
			expectedCollectorCount: 1,
		},
		{
			name:                   "disables collector on zero frequency",
			defaultConfigs:         CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
			existingColls:          collectors{fakeMD: {Resource: fakeRes, Collector: toggledCollector, Config: fakeCfg}},
			input:                  map[string]datamanager.CaptureConfigReading{"fake-1/GetReadings": {CaptureFrequencyHz: float32Ptr(0)}},
			expectedClosed:         toggledCollector,
			expectedCollectorCount: 0,
		},
		{
			name:                   "disables collector on near-zero frequency",
			defaultConfigs:         CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
			existingColls:          collectors{fakeMD: {Resource: fakeRes, Collector: nearZeroCollector, Config: fakeCfg}},
			input:                  map[string]datamanager.CaptureConfigReading{"fake-1/GetReadings": {CaptureFrequencyHz: float32Ptr(1e-7)}},
			expectedClosed:         nearZeroCollector,
			expectedCollectorCount: 0,
		},
		{
			name: "reverts to default config on nil input",
			defaultConfigs: CollectorConfigsByResource{
				fakeRes: []datamanager.DataCaptureConfig{
					{
						Name:               resource.NewName(fakeAPI, "fake-1"),
						Method:             "GetReadings",
						CaptureFrequencyHz: 1.0,
						Disabled:           true,
					},
				},
			},
			input:                  nil,
			expectedCollectorCount: 0,
		},
		{
			name:                   "service-level tags are overridden by capture config tags",
			defaultConfigs:         CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
			existingColls:          collectors{fakeMD: {Resource: fakeRes, Collector: tagsCollector, Config: fakeCfg}},
			defaultTags:            []string{"service-tag"},
			input:                  map[string]datamanager.CaptureConfigReading{"fake-1/GetReadings": {Tags: []string{"override-tag"}}},
			expectedClosed:         tagsCollector,
			expectedCollectorCount: 1,
			expectedNewTags:        []string{"override-tag"},
		},
		{
			name:           "enables capture for a resource not in defaultConfigs",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"fake-1": fakeRes},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": {
					ResourceMethod:     datamanager.ResourceMethod{ResourceName: "fake-1", MethodName: "GetReadings"},
					CaptureFrequencyHz: float32Ptr(1.0),
				},
			},
			expectedCollectorCount: 1,
		},
		{
			name:           "skips override for unknown resource",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{},
			input: map[string]datamanager.CaptureConfigReading{
				"unknown/GetReadings": {
					ResourceMethod:     datamanager.ResourceMethod{ResourceName: "unknown", MethodName: "GetReadings"},
					CaptureFrequencyHz: float32Ptr(1.0),
				},
			},
			expectedCollectorCount: 0,
		},
		{
			name:           "auto-enabled collector inherits service-level tags",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"fake-1": fakeRes},
			defaultTags:    []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": {
					ResourceMethod:     datamanager.ResourceMethod{ResourceName: "fake-1", MethodName: "GetReadings"},
					CaptureFrequencyHz: float32Ptr(1.0),
				},
			},
			expectedCollectorCount: 1,
			expectedNewTags:        []string{"service-tag"},
		},
		{
			name:           "skips auto-enable for method requiring additional_params",
			defaultConfigs: CollectorConfigsByResource{},
			catalog:        map[string]resource.Resource{"board-1": boardRes},
			input: map[string]datamanager.CaptureConfigReading{
				"board-1/Analogs": {
					ResourceMethod:     datamanager.ResourceMethod{ResourceName: "board-1", MethodName: "Analogs"},
					CaptureFrequencyHz: float32Ptr(1.0),
				},
			},
			expectedCollectorCount: 0,
		},
		{
			name:           "sensor-driven rebuild of static collector preserves service-level tags",
			defaultConfigs: CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
			existingColls: collectors{fakeMD: {
				Resource:  fakeRes,
				Collector: &mockCollector{},
				Config: datamanager.DataCaptureConfig{Name: fakeCfg.Name, Method: fakeCfg.Method,
					CaptureFrequencyHz: 1.0, Tags: []string{"service-tag"}},
			}},
			defaultTags: []string{"service-tag"},
			input: map[string]datamanager.CaptureConfigReading{
				"fake-1/GetReadings": {CaptureFrequencyHz: float32Ptr(5.0)},
			},
			expectedCollectorCount: 1,
			expectedNewTags:        []string{"service-tag"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registerFakeCollector()

			c := newTestCapture(t, tc.defaultConfigs, tc.existingColls, tc.catalog, tc.defaultTags)

			c.SetCaptureConfigs(context.Background(), tc.input)

			if tc.expectedClosed != nil {
				test.That(t, tc.expectedClosed.closed, test.ShouldBeTrue)
			}
			if tc.expectedNotClosed != nil {
				test.That(t, tc.expectedNotClosed.closed, test.ShouldBeFalse)
			}

			test.That(t, len(c.collectors), test.ShouldEqual, tc.expectedCollectorCount)

			if tc.expectedNewTags != nil {
				test.That(t, c.collectors[fakeMD].Config.Tags, test.ShouldResemble, tc.expectedNewTags)
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
	c.SetCaptureConfigs(context.Background(), override)
	test.That(t, len(c.collectors), test.ShouldEqual, 1)

	// Sensor stops returning the override; the orphan pass should close the collector.
	c.SetCaptureConfigs(context.Background(), nil)
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
