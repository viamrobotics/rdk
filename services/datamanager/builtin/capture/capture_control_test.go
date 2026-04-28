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
	"go.viam.com/rdk/testutils/inject"
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
	fakeAPI = resource.APINamespaceRDK.WithComponentType("fake")
	fakeRes = inject.NewSensor("fake-1")
)

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
func newTestCapture(
	t *testing.T,
	defaultCollectorConfigs CollectorConfigsByResource,
	existingCollectors collectors,
) *Capture {
	t.Helper()
	if existingCollectors == nil {
		existingCollectors = make(collectors)
	}
	return &Capture{
		logger:                  logging.NewTestLogger(t),
		clk:                     clock.New(),
		collectors:              existingCollectors,
		captureDir:              t.TempDir(),
		maxCaptureFileSize:      256 * 1024,
		defaultCollectorConfigs: defaultCollectorConfigs,
	}
}

func TestSetCaptureConfig(t *testing.T) {
	fakeCfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(fakeAPI, "fake-1"),
		Method:             "GetReadings",
		CaptureFrequencyHz: 1.0,
	}
	fakeMD := newCollectorMetadata(fakeCfg)

	toggledCollector := &mockCollector{}
	tagsCollector := &mockCollector{}
	noopCollector := &mockCollector{}

	for _, tc := range []struct {
		name                   string
		defaultConfigs         CollectorConfigsByResource
		existingColls          collectors
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			registerFakeCollector()

			c := newTestCapture(t, tc.defaultConfigs, tc.existingColls)

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
					CollectorConfigsByResource{fakeRes: []datamanager.DataCaptureConfig{fakeCfg}},
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
