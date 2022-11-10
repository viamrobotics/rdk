package board_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testBoardName    = "board1"
	testBoardName2   = "board2"
	fakeBoardName    = "board3"
	missingBoardName = "board4"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[board.Named(testBoardName)] = newLocalBoard(testBoardName)
	deps[board.Named(fakeBoardName)] = "not a board"
	return deps
}

func setupInjectRobot() *inject.Robot {
	board1 := newLocalBoard(testBoardName)
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case board.Named(testBoardName):
			return board1, nil
		case board.Named(fakeBoardName):
			return "not a board", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{board.Named(testBoardName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	b, err := board.FromRobot(r, testBoardName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := b.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := board.FromDependencies(deps, testBoardName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	p, err := res.(board.LocalBoard).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	result, err := p.Get(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, mockGPIO)

	res, err = board.FromDependencies(deps, fakeBoardName)
	test.That(t, err, test.ShouldBeError, board.DependencyTypeError(fakeBoardName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = board.FromDependencies(deps, missingBoardName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingBoardName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := board.FromRobot(r, testBoardName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	p, err := res.(board.LocalBoard).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	result, err := p.Get(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, mockGPIO)

	res, err = board.FromRobot(r, fakeBoardName)
	test.That(t, err, test.ShouldBeError, board.NewUnimplementedInterfaceError("string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = board.FromRobot(r, missingBoardName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(board.Named(missingBoardName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := board.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testBoardName})
}

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
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: board.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testBoardName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: board.SubtypeName,
				},
				Name: testBoardName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := board.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

var (
	mockStatus *commonpb.BoardStatus
	mockGPIO   bool
)

func TestWrapWithReconfigurable(t *testing.T) {
	var actualBoard board.Board = newLocalBoard(testBoardName)

	reconfBoard1, err := board.WrapWithReconfigurable(actualBoard)
	test.That(t, err, test.ShouldBeNil)

	_, err = board.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, board.NewUnimplementedInterfaceError(nil))

	reconfBoard2, err := board.WrapWithReconfigurable(reconfBoard1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfBoard2, test.ShouldEqual, reconfBoard1)

	var actualBoard2 board.LocalBoard = &mockLocal{Name: testBoardName}
	reconfBoard3, err := board.WrapWithReconfigurable(actualBoard2)
	test.That(t, err, test.ShouldBeNil)

	reconfBoard4, err := board.WrapWithReconfigurable(reconfBoard3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfBoard4, test.ShouldResemble, reconfBoard3)

	_, ok := reconfBoard4.(board.LocalBoard)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestReconfigurableBoard(t *testing.T) {
	actualBoards := []*mock{
		newBoard(testBoardName),
		newBareBoard(testBoardName),
	}

	for _, actualBoard1 := range actualBoards {
		reconfBoard1, err := board.WrapWithReconfigurable(actualBoard1)
		test.That(t, err, test.ShouldBeNil)

		actualBoard2 := newBoard(testBoardName2)
		reconfBoard2, err := board.WrapWithReconfigurable(actualBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 0)

		err = reconfBoard1.Reconfigure(context.Background(), reconfBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, reconfBoard1, test.ShouldResemble, reconfBoard2)
		test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 1)

		actualBoard3 := newLocalBoard(testBoardName2)
		reconfBoard3, err := board.WrapWithReconfigurable(actualBoard3)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualBoard3.reconfCount, test.ShouldEqual, 0)

		err = reconfBoard3.Reconfigure(context.Background(), reconfBoard2)
		test.That(t, err, test.ShouldBeError)
		test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfBoard3, reconfBoard1))

		actualBoard4 := &mock{Name: testBoardName2}
		reconfBoard4, err := board.WrapWithReconfigurable(actualBoard4)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, reconfBoard4, test.ShouldNotBeNil)
	}

	actualBoard1 := &mock{Name: testBoardName}
	reconfBoard1, err := board.WrapWithReconfigurable(actualBoard1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfBoard1, test.ShouldNotBeNil)
}

func TestReconfigurableLocalBoard(t *testing.T) {
	actualBoards := []*mockLocal{
		newLocalBoard(testBoardName),
		newBareLocalBoard(testBoardName),
	}

	for _, actualBoard1 := range actualBoards {
		reconfBoard1, err := board.WrapWithReconfigurable(actualBoard1)
		test.That(t, err, test.ShouldBeNil)

		actualBoard2 := newLocalBoard(testBoardName2)
		reconfBoard2, err := board.WrapWithReconfigurable(actualBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 0)

		err = reconfBoard1.Reconfigure(context.Background(), reconfBoard2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, reconfBoard1, test.ShouldResemble, reconfBoard2)
		test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 2)

		test.That(t, actualBoard1.gpioPin.getCount, test.ShouldEqual, 0)
		test.That(t, actualBoard2.gpioPin.getCount, test.ShouldEqual, 0)

		p, err := reconfBoard1.(board.Board).GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		result, err := p.Get(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, mockGPIO)
		test.That(t, actualBoard1.gpioPin.getCount, test.ShouldEqual, 0)
		test.That(t, actualBoard2.gpioPin.getCount, test.ShouldEqual, 1)

		err = reconfBoard1.Reconfigure(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfBoard1, nil))
	}
}

func TestSetGPIO(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioPin.setCount, test.ShouldEqual, 0)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldBeNil)
	p, err := reconfBoard.(board.Board).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	err = p.Set(context.Background(), false, extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.gpioPin.setCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldResemble, extra)
}

func TestGetGPIO(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioPin.getCount, test.ShouldEqual, 0)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldBeNil)
	p, err := reconfBoard.(board.Board).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	result, err := p.Get(context.Background(), extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, mockGPIO)
	test.That(t, actualBoard.gpioPin.getCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldResemble, extra)
}

func TestSetPWM(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioPin.setPWMCount, test.ShouldEqual, 0)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldBeNil)
	p, err := reconfBoard.(board.Board).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	err = p.SetPWM(context.Background(), 0, extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.gpioPin.setPWMCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldResemble, extra)
}

func TestSetPWMFreq(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.gpioPin.setPWMFreqCount, test.ShouldEqual, 0)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldBeNil)
	p, err := reconfBoard.(board.Board).GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	err = p.SetPWMFreq(context.Background(), 0, extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.gpioPin.setPWMFreqCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.gpioPin.extra, test.ShouldResemble, extra)
}

func TestStatus(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	test.That(t, actualBoard.statusCount, test.ShouldEqual, 0)
	test.That(t, actualBoard.extra, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	status, err := reconfBoard.(board.LocalBoard).Status(context.Background(), extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, mockStatus)
	test.That(t, actualBoard.statusCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.extra, test.ShouldResemble, extra)
}

func TestSPIs(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	reconfSPINames := reconfBoard.(board.LocalBoard).SPINames()
	test.That(t, reconfSPINames, test.ShouldResemble, []string{"spi1"})

	reconfSPI, ok := reconfBoard.(board.LocalBoard).SPIByName("spi1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err := reconfSPI.OpenHandle()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.spi.handleCount, test.ShouldEqual, 1)
}

func TestI2Cs(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	reconfI2CNames := reconfBoard.(board.LocalBoard).I2CNames()
	test.That(t, reconfI2CNames, test.ShouldResemble, []string{"i2c1"})

	reconfI2C, ok := reconfBoard.(board.LocalBoard).I2CByName("i2c1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err := reconfI2C.OpenHandle(0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.i2c.handleCount, test.ShouldEqual, 1)
}

//nolint:dupl
func TestAnalogReaders(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	reconfAnalogReaderNames := reconfBoard.(board.LocalBoard).AnalogReaderNames()
	test.That(t, reconfAnalogReaderNames, test.ShouldResemble, []string{"analog1"})

	reconfAnalogReader, ok := reconfBoard.(board.LocalBoard).AnalogReaderByName("analog1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, actualBoard.analog.extra, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	_, err := reconfAnalogReader.Read(context.Background(), extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.analog.readCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.analog.extra, test.ShouldResemble, extra)
}

//nolint:dupl
func TestDigitalInterrupts(t *testing.T) {
	actualBoard := newLocalBoard(testBoardName)
	reconfBoard, _ := board.WrapWithReconfigurable(actualBoard)

	reconfDigitalInterruptNames := reconfBoard.(board.LocalBoard).DigitalInterruptNames()
	test.That(t, reconfDigitalInterruptNames, test.ShouldResemble, []string{"digital1"})

	reconfDigitalInterrupt, ok := reconfBoard.(board.LocalBoard).DigitalInterruptByName("digital1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, actualBoard.digital.extra, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}}
	_, err := reconfDigitalInterrupt.Value(context.Background(), extra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard.digital.valueCount, test.ShouldEqual, 1)
	test.That(t, actualBoard.digital.extra, test.ShouldResemble, extra)
}

func TestClose(t *testing.T) {
	actualBoard1 := &mockLocal{Name: testBoardName}
	reconfBoard1, _ := board.WrapWithReconfigurable(actualBoard1)

	test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfBoard1), test.ShouldBeNil)
	test.That(t, actualBoard1.reconfCount, test.ShouldEqual, 1)
}

type mock struct {
	board.Board
	Name     string
	spis     []string
	i2cs     []string
	analogs  []string
	digitals []string
	gpioPins []string

	spi     *mockSPI
	i2c     *mockI2C
	analog  *mockAnalogReader
	digital *mockDigitalInterrupt
	gpioPin *mockGPIOPin

	reconfCount int
}

func newBoard(name string) *mock {
	return &mock{
		Name:     name,
		i2cs:     []string{"i2c1"},
		spis:     []string{"spi1"},
		analogs:  []string{"analog1"},
		digitals: []string{"digital1"},
		gpioPins: []string{"1"},
		i2c:      &mockI2C{},
		spi:      &mockSPI{},
		analog:   &mockAnalogReader{},
		digital:  &mockDigitalInterrupt{},
		gpioPin:  &mockGPIOPin{},
	}
}

// A board without any subcomponents.
func newBareBoard(name string) *mock {
	return &mock{Name: name, gpioPin: &mockGPIOPin{}}
}

func (m *mock) Close() error { m.reconfCount++; return nil }

type mockLocal struct {
	board.LocalBoard
	Name string

	spis     []string
	i2cs     []string
	analogs  []string
	digitals []string
	gpioPins []string

	spi     *mockSPI
	i2c     *mockI2C
	analog  *mockAnalogReader
	digital *mockDigitalInterrupt
	gpioPin *mockGPIOPin

	reconfCount int
	statusCount int

	extra map[string]interface{}
}

func (m *mock) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	if len(m.analogs) == 0 {
		return nil, false
	}
	return m.analog, true
}

func (m *mock) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	if len(m.digitals) == 0 {
		return nil, false
	}
	return m.digital, true
}

func (m *mock) AnalogReaderNames() []string {
	return m.analogs
}

func (m *mock) DigitalInterruptNames() []string {
	return m.digitals
}

// Helpers

func newLocalBoard(name string) *mockLocal {
	return &mockLocal{
		Name:     name,
		i2cs:     []string{"i2c1"},
		spis:     []string{"spi1"},
		analogs:  []string{"analog1"},
		digitals: []string{"digital1"},
		gpioPins: []string{"1"},
		i2c:      &mockI2C{},
		spi:      &mockSPI{},
		analog:   &mockAnalogReader{},
		digital:  &mockDigitalInterrupt{},
		gpioPin:  &mockGPIOPin{},
	}
}

// A board without any subcomponents.
func newBareLocalBoard(name string) *mockLocal {
	return &mockLocal{Name: name, gpioPin: &mockGPIOPin{}}
}

// Interface methods

func (m *mockLocal) SPINames() []string {
	return m.spis
}

func (m *mockLocal) I2CNames() []string {
	return m.i2cs
}

func (m *mockLocal) AnalogReaderNames() []string {
	return m.analogs
}

func (m *mockLocal) DigitalInterruptNames() []string {
	return m.digitals
}

func (m *mockLocal) SPIByName(name string) (board.SPI, bool) {
	if len(m.spis) == 0 {
		return nil, false
	}
	return m.spi, true
}

func (m *mockLocal) I2CByName(name string) (board.I2C, bool) {
	if len(m.i2cs) == 0 {
		return nil, false
	}
	return m.i2c, true
}

func (m *mockLocal) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	if len(m.analogs) == 0 {
		return nil, false
	}
	return m.analog, true
}

func (m *mockLocal) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	if len(m.digitals) == 0 {
		return nil, false
	}
	return m.digital, true
}

func (m *mockLocal) GPIOPinByName(name string) (board.GPIOPin, error) {
	if len(m.gpioPins) == 0 {
		return nil, errors.New("no pin")
	}
	return m.gpioPin, nil
}

func (m *mockLocal) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{Remote: true}
}

func (m *mockLocal) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (m *mockLocal) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	m.statusCount++
	m.extra = extra
	return mockStatus, nil
}

func (m *mockLocal) Close() error { m.reconfCount++; return nil }

type mockGPIOPin struct {
	setCount, getCount, pwmCount, setPWMCount, pwmFreqCount, setPWMFreqCount int
	extra                                                                    map[string]interface{}
}

func (gp *mockGPIOPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	gp.setCount++
	gp.extra = extra
	return nil
}

func (gp *mockGPIOPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	gp.getCount++
	gp.extra = extra
	return mockGPIO, nil
}

func (gp *mockGPIOPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	gp.pwmCount++
	gp.extra = extra
	return 23, nil
}

func (gp *mockGPIOPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	gp.setPWMCount++
	gp.extra = extra
	return nil
}

func (gp *mockGPIOPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	gp.pwmFreqCount++
	gp.extra = extra
	return 42, nil
}

func (gp *mockGPIOPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	gp.setPWMFreqCount++
	gp.extra = extra
	return nil
}

// Mock SPI

type mockSPI struct{ handleCount int }

func (m *mockSPI) OpenHandle() (board.SPIHandle, error) {
	m.handleCount++
	return &mockSPIHandle{}, nil
}

type mockSPIHandle struct{}

func (m *mockSPIHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockSPIHandle) Close() error { return nil }

// Mock I2C

type mockI2C struct{ handleCount int }

func (m *mockI2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	m.handleCount++
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

func (m *mockI2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return nil
}

func (m *mockI2CHandle) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	return 0, nil
}

func (m *mockI2CHandle) WriteWordData(ctx context.Context, register byte, data uint16) error {
	return nil
}

func (m *mockI2CHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockI2CHandle) WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error {
	return nil
}

func (m *mockI2CHandle) Close() error { return nil }

// Mock AnalogReader

type mockAnalogReader struct {
	readCount int
	extra     map[string]interface{}
}

func (m *mockAnalogReader) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	m.readCount++
	m.extra = extra
	return 0, nil
}

// Mock DigitalInterrupt

type mockDigitalInterrupt struct {
	valueCount int
	extra      map[string]interface{}
}

func (m *mockDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	m.valueCount++
	m.extra = extra
	return 0, nil
}

func (m *mockDigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	return nil
}
func (m *mockDigitalInterrupt) AddCallback(c chan bool)                 {}
func (m *mockDigitalInterrupt) RemoveCallback(c chan bool)              {}
func (m *mockDigitalInterrupt) AddPostProcessor(pp board.PostProcessor) {}
