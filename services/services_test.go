// add tests here or somewhere else that makes sure normal services turn into reconfigurable ones.

package services

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/rdk/resource"
	"go.viam.com/test"
)

func TestServiceWrapWithReconfigure(t *testing.T) {
	actualA := &mock{a: 1}
	reconfigA, err := WrapWithReconfigurable(actualA)
	test.That(t, err, test.ShouldBeNil)

	reconfReconfigA := reconfigA.(*ReconfigurableService)
	fmt.Println(reconfReconfigA.Actual)
	// test.That(t, ok, test.ShouldBeTrue)
	actualActual := reconfReconfigA.Actual

	fmt.Println(&actualActual)
	// mockActual, ok := (*actualActual).(Mock)

	// actualAA := *actualActual

	actualB := &mock{a: 2}
	reconfigB, err := WrapWithReconfigurable(actualB)
	test.That(t, err, test.ShouldBeNil)

	reconfReconfigB, _ := reconfigB.(*ReconfigurableService)
	fmt.Println(reconfReconfigB.Actual)
	actualActualB := *(reconfReconfigB.Actual)

	reconfReconfigA.Reconfigure(context.Background(), reconfReconfigB)
	fmt.Println(reconfReconfigA.Actual)
	fmt.Println(*(reconfReconfigA.Actual))
	mockActual := (actualActual).(*mock)
	mockActualB := (actualActualB).(*mock)

	test.That(t, mockActual.a, test.ShouldEqual, mockActualB.a)

}

type mock struct {
	a int
	resource.Updateable
}

func (m *mock) Get() int {
	return m.a
}

type Mock interface {
	Get() int
}
