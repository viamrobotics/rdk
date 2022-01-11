package board

import (
	"context"
	"testing"

	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
)

func TestBoardName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "9596b6cc-dc7f-52b6-a050-0a9a09e3f90d",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"board1",
			resource.Name{
				UUID: "211f7735-3e34-563e-a01b-420c58b5a974",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "board1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

var (
	mockStatus *commonpb.BoardStatus
	mockGPIO   bool
)

func TestWrapWithReconfigurable(t *testing.T) {
	var actualBoard Board = newBoard("board1")

	fakeBoard1, err := WrapWithReconfigurable(actualBoard)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeBoard2, err := WrapWithReconfigurable(fakeBoard1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard2, test.ShouldEqual, fakeBoard1)
}

func TestReconfigurableBoard(t *testing.T) {
	actualBoards := []*mock{
		newBoard("board1"),
		newBareBoard("boards"),
	}

	for _, actualBoard1 := range actualBoards {
		fakeBoard1, err := WrapWithReconfigurable(actualBoard1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard1)
		err = fakeBoard1.(*reconfigurableBoard).Reconfigure(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "expected new board")
		test.That(t, actualBoard1.reconfCalls, test.ShouldEqual, 0)

		actualBoard2 := newBoard("board2")
		fakeBoard2, err := WrapWithReconfigurable(actualBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualBoard1.reconfCalls, test.ShouldEqual, 0)

		err = fakeBoard1.(*reconfigurableBoard).Reconfigure(context.Background(), fakeBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard2)
		test.That(t, actualBoard1.reconfCalls, test.ShouldEqual, 1)
	}
}

func TestGPIOSet(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioSetCalls, test.ShouldEqual, 0)
	err := fakeBoard.(*reconfigurableBoard).GPIOSet(context.Background(), "", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.gpioSetCalls, test.ShouldEqual, 1)
}

func TestGPIOGet(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioGetCalls, test.ShouldEqual, 0)
	result, err := fakeBoard.(*reconfigurableBoard).GPIOGet(context.Background(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, mockGPIO)
	test.That(t, actualBoard.gpioGetCalls, test.ShouldEqual, 1)
}

func TestPWMSet(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.pwmSetCalls, test.ShouldEqual, 0)
	err := fakeBoard.(*reconfigurableBoard).PWMSet(context.Background(), "", 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.pwmSetCalls, test.ShouldEqual, 1)
}

func TestPWMSetFreq(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.pwmSetFreqCalls, test.ShouldEqual, 0)
	err := fakeBoard.(*reconfigurableBoard).PWMSetFreq(context.Background(), "", 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.pwmSetFreqCalls, test.ShouldEqual, 1)
}

func TestStatus(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.statusCalls, test.ShouldEqual, 0)
	status, err := fakeBoard.(*reconfigurableBoard).Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, mockStatus)
	test.That(t, actualBoard.statusCalls, test.ShouldEqual, 1)
}

func TestSPIs(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	fakeSPINames := fakeBoard.(*reconfigurableBoard).SPINames()
	test.That(t, fakeSPINames, test.ShouldResemble, []string{"spi1"})

	fakeSPI, ok := fakeBoard.(*reconfigurableBoard).SPIByName("spi1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fakeSPI, test.ShouldResemble, &reconfigurableSPI{actual: &mockSPI{}})
}

func TestI2Cs(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	fakeI2CNames := fakeBoard.(*reconfigurableBoard).I2CNames()
	test.That(t, fakeI2CNames, test.ShouldResemble, []string{"i2c1"})

	fakeI2C, ok := fakeBoard.(*reconfigurableBoard).I2CByName("i2c1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fakeI2C, test.ShouldResemble, &reconfigurableI2C{actual: &mockI2C{}})
}

func TestAnalogReaders(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	fakeAnalogReaderNames := fakeBoard.(*reconfigurableBoard).AnalogReaderNames()
	test.That(t, fakeAnalogReaderNames, test.ShouldResemble, []string{"analog1"})

	fakeAnalogReader, ok := fakeBoard.(*reconfigurableBoard).AnalogReaderByName("analog1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fakeAnalogReader, test.ShouldResemble, &reconfigurableAnalogReader{actual: &mockAnalogReader{}})
}

func TestDigitalInterrupts(t *testing.T) {
	actualBoard := newBoard("board1")
	fakeBoard, _ := WrapWithReconfigurable(actualBoard)

	fakeDigitalInterruptNames := fakeBoard.(*reconfigurableBoard).DigitalInterruptNames()
	test.That(t, fakeDigitalInterruptNames, test.ShouldResemble, []string{"digital1"})

	fakeDigitalInterrupt, ok := fakeBoard.(*reconfigurableBoard).DigitalInterruptByName("digital1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fakeDigitalInterrupt, test.ShouldResemble, &reconfigurableDigitalInterrupt{actual: &mockDigitalInterrupt{}})
}

type mock struct {
	Board
	Name string

	spis     []string
	i2cs     []string
	analogs  []string
	digitals []string

	reconfCalls     int
	gpioSetCalls    int
	gpioGetCalls    int
	pwmSetCalls     int
	pwmSetFreqCalls int
	statusCalls     int
}

// Helpers

func newBoard(name string) *mock {
	return &mock{
		Name:     name,
		i2cs:     []string{"i2c1"},
		spis:     []string{"spi1"},
		analogs:  []string{"analog1"},
		digitals: []string{"digital1"},
	}
}

// A board without any subcomponents.
func newBareBoard(name string) *mock {
	return &mock{Name: name}
}

// Interface methods

func (m *mock) SPINames() []string {
	return m.spis
}

func (m *mock) I2CNames() []string {
	return m.i2cs
}

func (m *mock) AnalogReaderNames() []string {
	return m.analogs
}

func (m *mock) DigitalInterruptNames() []string {
	return m.digitals
}

func (m *mock) SPIByName(name string) (SPI, bool) {
	if len(m.spis) == 0 {
		return nil, false
	}
	return &mockSPI{}, true
}

func (m *mock) I2CByName(name string) (I2C, bool) {
	if len(m.i2cs) == 0 {
		return nil, false
	}
	return &mockI2C{}, true
}

func (m *mock) AnalogReaderByName(name string) (AnalogReader, bool) {
	if len(m.analogs) == 0 {
		return nil, false
	}
	return &mockAnalogReader{}, true
}

func (m *mock) DigitalInterruptByName(name string) (DigitalInterrupt, bool) {
	if len(m.digitals) == 0 {
		return nil, false
	}
	return &mockDigitalInterrupt{}, true
}

func (m *mock) ModelAttributes() ModelAttributes {
	return ModelAttributes{Remote: true}
}

func (m *mock) GPIOSet(ctx context.Context, pin string, high bool) error {
	m.gpioSetCalls++
	return nil
}

func (m *mock) GPIOGet(ctx context.Context, pin string) (bool, error) {
	m.gpioGetCalls++
	return mockGPIO, nil
}

func (m *mock) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	m.pwmSetCalls++
	return nil
}

func (m *mock) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	m.pwmSetFreqCalls++
	return nil
}

func (m *mock) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	m.statusCalls++
	return mockStatus, nil
}

func (m *mock) Close() error { m.reconfCalls++; return nil }

// Mock SPI

type mockSPI struct{}

func (m *mockSPI) OpenHandle() (SPIHandle, error) {
	return &mockSPIHandle{}, nil
}

type mockSPIHandle struct{}

func (m *mockSPIHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockSPIHandle) Close() error { return nil }

// Mock I2C

type mockI2C struct{}

func (m *mockI2C) OpenHandle(addr byte) (I2CHandle, error) {
	return &mockI2CHandle{}, nil
}

type mockI2CHandle struct{}

func (m *mockI2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockI2CHandle) Write(ctx context.Context, tx []byte) error {
	return nil
}

func (m *mockI2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, nil
}

func (m *mockI2CHandle) WriteByteData(ctx context.Context, register byte, data byte) error {
	return nil
}

func (m *mockI2CHandle) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	return 0, nil
}

func (m *mockI2CHandle) WriteWordData(ctx context.Context, register byte, data uint16) error {
	return nil
}
func (m *mockI2CHandle) Close() error { return nil }

// Mock AnalogReader

type mockAnalogReader struct{}

func (m *mockAnalogReader) Read(ctx context.Context) (int, error) {
	return 0, nil
}

// Mock DigitalInterrupt

type mockDigitalInterrupt struct{}

func (m *mockDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	return DigitalInterruptConfig{}, nil
}

func (m *mockDigitalInterrupt) Value(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	return nil
}
func (m *mockDigitalInterrupt) AddCallback(c chan bool)           {}
func (m *mockDigitalInterrupt) AddPostProcessor(pp PostProcessor) {}
