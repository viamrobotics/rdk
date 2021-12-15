package board

import (
	"context"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"

	"go.viam.com/test"
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
				UUID: "b957b292-d6a8-5dc1-9cbe-12db3a623972",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"board1",
			resource.Name{
				UUID: "98714ab0-2538-52c3-b378-0ae616900d20",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
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

// var (
// 	mockStatus *pb.BoardStatus
// )

func TestWrapWithReconfigurable(t *testing.T) {
	var actualBoard Board = &mock{Name: "board1"}

	// Wrap an actual board with reconfigurable
	fakeBoard1, err := WrapWithReconfigurable(actualBoard)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard)

	// Wrap `nil` with reconfigurable
	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	// Wrap a reconfigurable board with reconfigurable
	fakeBoard2, err := WrapWithReconfigurable(fakeBoard1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard2, test.ShouldEqual, fakeBoard1)
}

func TestReconfigurableBoard(t *testing.T) {
	actualBoard1 := &mock{Name: "board1"}
	fakeBoard1, err := WrapWithReconfigurable(actualBoard1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard1)

	actualBoard2 := &mock{Name: "board2"}
	fakeBoard2, err := WrapWithReconfigurable(actualBoard2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualBoard1.reconfCalls, test.ShouldEqual, 0)

	err = fakeBoard1.(*reconfigurableBoard).Reconfigure(fakeBoard2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeBoard1.(*reconfigurableBoard).actual, test.ShouldEqual, actualBoard2)
	test.That(t, actualBoard1.reconfCalls, test.ShouldEqual, 1)

	err = fakeBoard1.(*reconfigurableBoard).Reconfigure(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new board")
}

// func TestStatus(t *testing.T) {
// 	actualBoard := &mock{Name: "board1"}
// 	fakeBoard, _ := WrapWithReconfigurable(actualBoard)
//
// 	test.That(t, actualBoard.statusCalls, test.ShouldEqual, 0)
// 	status, err := fakeBoard.(*reconfigurableBoard).Status(context.Background())
// 	test.That(t, err, test.ShouldBeNil)
// 	test.That(t, status, test.ShouldResemble, mockStatus)
// 	test.That(t, actualBoard.statusCalls, test.ShouldEqual, 1)
// }

type mock struct {
	Board
	Name            string
	// ModelAttributes struct{ Remote bool }
	reconfCalls     int
	statusCalls     int
}

// TODO(maximpertsov): add board subcomponents
func (m *mock) SPINames() []string {
	// return []string{"spi1"}
	return []string{}
}
func (m *mock) I2CNames() []string {
	// return []string{"i2c1"}
	return []string{}
}
func (m *mock) AnalogReaderNames() []string {
	// return []string{"analog1"}
	return []string{}
}
func (m *mock) DigitalInterruptNames() []string {
	// return []string{"digital1"}
	return []string{}
}

// TODO(maximpertsov): add board subcomponents
// func (m *mock) SPIByName(name string) (SPI, bool) {
// 	return inject.SPI{}, true
// }
// func (m *mock) I2CByNameFunc(name string) (I2C, bool) {
// 	return inject.I2C{}, true
// }
// func (m *mock) AnalogReaderByNameFunc(name string) (AnalogReader, bool) {
// 	return &fake.Analog{}, true
// }
// func (m *mock) DigitalInterruptByNameFunc(name string) (DigitalInterrupt, bool) {
// 	return &BasicDigitalInterrupt{}, true
// }

func (m *mock) Status(ctx context.Context) (*pb.BoardStatus, error) {
	m.statusCalls++
	return nil, nil
}

func (m *mock) Close() error { m.reconfCalls++; return nil }
