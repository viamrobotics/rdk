package audioinput_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testAudioInputName    = "mic1"
	testAudioInputName2   = "mic2"
	failAudioInputName    = "mic3"
	fakeAudioInputName    = "mic4"
	missingAudioInputName = "mic5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[audioinput.Named(testAudioInputName)] = &mock{Name: testAudioInputName}
	deps[audioinput.Named(fakeAudioInputName)] = "not an audio input"
	return deps
}

func setupInjectRobot() *inject.Robot {
	audioInput1 := &mock{Name: testAudioInputName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case audioinput.Named(testAudioInputName):
			return audioInput1, nil
		case audioinput.Named(fakeAudioInputName):
			return "not an audio input", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{audioinput.Named(testAudioInputName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	c, err := audioinput.FromRobot(r, testAudioInputName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := c.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := audioinput.FromDependencies(deps, testAudioInputName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	audioData1, _, err := gostream.ReadAudio(context.Background(), res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, audioData, test.ShouldResemble, audioData1)
	test.That(t, res.Close(context.Background()), test.ShouldBeNil)

	res, err = audioinput.FromDependencies(deps, fakeAudioInputName)
	test.That(t, err, test.ShouldBeError, audioinput.DependencyTypeError(fakeAudioInputName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = audioinput.FromDependencies(deps, missingAudioInputName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingAudioInputName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := audioinput.FromRobot(r, testAudioInputName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	audioData1, _, err := gostream.ReadAudio(context.Background(), res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, audioData, test.ShouldResemble, audioData1)
	test.That(t, res.Close(context.Background()), test.ShouldBeNil)

	res, err = audioinput.FromRobot(r, fakeAudioInputName)
	test.That(t, err, test.ShouldBeError, audioinput.NewUnimplementedInterfaceError("string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = audioinput.FromRobot(r, missingAudioInputName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(audioinput.Named(missingAudioInputName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := audioinput.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testAudioInputName})
}

func TestAudioInputName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: audioinput.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testAudioInputName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: audioinput.SubtypeName,
				},
				Name: testAudioInputName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := audioinput.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualaudioInput1 audioinput.AudioInput = &mock{Name: testAudioInputName}
	reconfaudioInput1, err := audioinput.WrapWithReconfigurable(actualaudioInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = audioinput.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, audioinput.NewUnimplementedInterfaceError(nil))

	reconfAudioInput2, err := audioinput.WrapWithReconfigurable(reconfaudioInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfAudioInput2, test.ShouldEqual, reconfaudioInput1)
}

func TestReconfigurableAudioInput(t *testing.T) {
	actualaudioInput1 := &mock{Name: testAudioInputName}
	reconfaudioInput1, err := audioinput.WrapWithReconfigurable(actualaudioInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualAudioInput2 := &mock{Name: testAudioInputName2}
	reconfAudioInput2, err := audioinput.WrapWithReconfigurable(actualAudioInput2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualaudioInput1.reconfCount, test.ShouldEqual, 0)

	stream, err := reconfaudioInput1.(audioinput.AudioInput).Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	nextImg, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextImg, test.ShouldResemble, audioData)

	err = reconfaudioInput1.Reconfigure(context.Background(), reconfAudioInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rutils.UnwrapProxy(reconfaudioInput1), test.ShouldResemble, rutils.UnwrapProxy(reconfAudioInput2))
	test.That(t, actualaudioInput1.reconfCount, test.ShouldEqual, 1)
	test.That(t, actualaudioInput1.nextCount, test.ShouldEqual, 1)
	test.That(t, actualAudioInput2.nextCount, test.ShouldEqual, 0)

	nextImg, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextImg, test.ShouldResemble, audioData)

	audioData1, _, err := gostream.ReadAudio(context.Background(), reconfaudioInput1.(audioinput.AudioInput))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, audioData, test.ShouldResemble, audioData1)
	test.That(t, actualaudioInput1.nextCount, test.ShouldEqual, 1)
	test.That(t, actualAudioInput2.nextCount, test.ShouldEqual, 2)
	test.That(t, reconfaudioInput1.(audioinput.AudioInput).Close(context.Background()), test.ShouldBeNil)

	err = reconfaudioInput1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *audioinput.reconfigurableAudioInput")
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
}

func TestClose(t *testing.T) {
	actualaudioInput1 := &mock{Name: testAudioInputName}
	reconfaudioInput1, err := audioinput.WrapWithReconfigurable(actualaudioInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualaudioInput1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfaudioInput1), test.ShouldBeNil)
	test.That(t, actualaudioInput1.reconfCount, test.ShouldEqual, 1)
}

var audioData = &wave.Float32Interleaved{
	Data: []float32{
		0.1, -0.5, 0.2, -0.6, 0.3, -0.7, 0.4, -0.8, 0.5, -0.9, 0.6, -1.0, 0.7, -1.1, 0.8, -1.2,
	},
	Size: wave.ChunkInfo{8, 2, 48000},
}

type mock struct {
	audioinput.AudioInput
	mu          sync.Mutex
	Name        string
	nextCount   int
	reconfCount int
	closedErr   error
}

type mockStream struct {
	m *mock
}

func (ms *mockStream) Next(ctx context.Context) (wave.Audio, func(), error) {
	ms.m.mu.Lock()
	defer ms.m.mu.Unlock()
	if ms.m.closedErr != nil {
		return nil, nil, ms.m.closedErr
	}
	ms.m.nextCount++
	return audioData, func() {}, nil
}

func (ms *mockStream) Close(ctx context.Context) error {
	ms.m.mu.Lock()
	defer ms.m.mu.Unlock()
	return nil
}

func (m *mock) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.AudioStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &mockStream{m: m}, nil
}

func (m *mock) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconfCount++
	m.closedErr = context.Canceled
	return nil
}

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type simpleSource struct{}

func (s *simpleSource) Read(ctx context.Context) (wave.Audio, func(), error) {
	return audioData, func() {}, nil
}

func TestNewAudioInput(t *testing.T) {
	audioSrc := &simpleSource{}

	_, err := audioinput.NewFromSource(nil)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil audio source"))

	_, err = audioinput.NewFromReader(nil, prop.Audio{})
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil reader"))

	audioInput1, err := audioinput.NewFromReader(audioSrc, prop.Audio{})
	test.That(t, err, test.ShouldBeNil)

	audioData1, _, err := gostream.ReadAudio(context.Background(), audioInput1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, audioData, test.ShouldResemble, audioData1)

	// audioInput1 wrapped with reconfigurable
	_, err = audioinput.WrapWithReconfigurable(audioInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, audioInput1.Close(context.Background()), test.ShouldBeNil)
}
