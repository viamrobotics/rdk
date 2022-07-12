// add tests here or somewhere else that makes sure normal services turn into reconfigurable ones.

package services

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestPointers(t *testing.T) {
	var count int = 4
	fmt.Println(count)

	var pv = &count
	*pv = 3
	fmt.Println(pv)
	fmt.Println(*pv)

	var pv2 *int = &count
	*pv = 2
	fmt.Println(pv2)
	fmt.Println(*pv2)

	pv3 := &count
	*pv = 1
	fmt.Println(pv3)
	fmt.Println(*pv3)
}

type mock2 struct {
	a int
}

func (m mock2) Get() int {
	return m.a
}

type Mo interface {
	Get() int
}

type mockMock struct {
	actual *mock2
}

type mockMock2 struct {
	actual *interface{}
}

type mock3 struct {
	num1 int
	num2 float64
	num3 []int
}

func TestM(t *testing.T) {
	var m = mock2{a: 1}

	var mInt interface{} = &m
	fmt.Printf("m: %v\n", m)
	fmt.Printf("m: %p\n", &m)
	fmt.Printf("mInt: %v\n", mInt)
	fmt.Printf("mInt: %p\n", &mInt)
	m.a = 5
	fmt.Printf("m: %v\n", m)
	fmt.Printf("m: %p\n", &m)
	fmt.Printf("mInt: %v\n", mInt)
	fmt.Printf("mInt: %p\n", &mInt)

	mockMock3 := mockMock2{actual: &mInt}
	actual3 := mockMock3.actual
	fmt.Printf("actual3: %v\n", actual3)
	fmt.Printf("actual3: %p\n", actual3)
	b := (*actual3)
	fmt.Printf("b: %v\n", b)
	fmt.Printf("b: %p\n", &b)
	a := (b).(*mock2)
	fmt.Printf("a: %v\n", a)
	fmt.Printf("a: %p\n", &a)

	actualA := (*mockMock3.actual).(*mock2)
	fmt.Printf("actualA: %v\n", actualA)
	// fmt.Printf("actualA: %p\n", actualA)

	var m2 interface{} = &mock2{a: 2}
	fmt.Printf("m %p\n", &m)
	fmt.Printf("m2 %p\n", &m2)

	mockMock1 := mockMock{actual: &m}
	fmt.Printf("mockMock1 %p\n", &mockMock1)
	fmt.Printf("%p\n", &mockMock3)
	fmt.Printf("mockMock1.actual %v\n", mockMock1.actual)
	fmt.Printf("mockMock1.actual %p\n", mockMock1.actual)
	fmt.Printf("mockMock3.actual %p\n", mockMock3.actual)

	g := mockMock1.actual
	fmt.Printf("g.Get(): %v\n", g.Get())

	// *mockMock1.actual = *m2.(*mock2)
	m2Ptr := (*mockMock3.actual)
	m2Ptr = m2
	// m2Ptr := (*mockMock3.actual).(*mock2)
	// *m2Ptr = *m2.(*mock2)
	fmt.Printf("m2Ptr: %v\n", m2Ptr)
	fmt.Printf("mockMock1.actual %v\n", mockMock1.actual)
	fmt.Printf("mockMock3.actual %p\n", mockMock3.actual)
	fmt.Printf("mockMock3.actual %v\n", (*mockMock3.actual).(*mock2))

	fmt.Printf("m: %v\n", m)
	fmt.Printf("actual3: %v\n", actual3)
	fmt.Printf("a: %v\n", a)
	// fmt.Printf("a: %p\n", a)

	fmt.Printf("g.Get(): %v\n", g.Get())

	// b = (*actual3)
	// fmt.Printf("b: %v\n", b)
	// fmt.Printf("b: %p\n", &b)
	// a = (b).(mock2)
	// fmt.Printf("a: %v\n", a)
	// fmt.Printf("a: %p\n", &a)

	fmt.Printf("actualA: %v\n", actualA)
	// fmt.Printf("actualA: %p\n", actualA)
}

func TestServiceWrapWithReconfigure(t *testing.T) {
	actualA := mock{a: 1}
	reconfigA, err := WrapWithReconfigurable(actualA) // this is already a pointer
	test.That(t, err, test.ShouldBeNil)

	fmt.Println("test if allocating to interface changes")
	reconfReconfigA := reconfigA.(*ReconfigurableService) // this is the part that gets fucked up
	//fmt.Println(reconfReconfigA.Actual)
	fmt.Println(&reconfReconfigA)
	fmt.Println(&(reconfReconfigA.Actual))

	reconfReconfigC := reconfigA.(*ReconfigurableService) // this is the part that gets fucked up
	//fmt.Println(reconfReconfigB.Actual)
	fmt.Println(&reconfReconfigC)
	fmt.Println(&(reconfReconfigC.Actual))

	fmt.Println("end test")
	// test.That(t, ok, test.ShouldBeTrue)
	actualActual := reconfReconfigA.Actual // reference to pointer

	fmt.Println(&actualActual)
	// mockActual, ok := (*actualActual).(Mock)

	// actualAA := *actualActual

	actualB := mock{a: 2}
	reconfigB, err := WrapWithReconfigurable(actualB)
	test.That(t, err, test.ShouldBeNil)

	reconfReconfigB, _ := reconfigB.(*ReconfigurableService)
	fmt.Println(reconfReconfigB.Actual)
	actualActualB := reconfReconfigB.Actual

	reconfReconfigA.Reconfigure(context.Background(), reconfReconfigB)
	fmt.Println(reconfReconfigA.Actual)
	fmt.Println(*(reconfReconfigA.Actual))
	mockActual := (*actualActual).(mock)
	mockActualB := (*actualActualB).(mock)

	test.That(t, mockActual.a, test.ShouldEqual, mockActualB.a)
	test.That(t, actualActual, test.ShouldEqual, actualActualB)

}

func TestDiffSizes(t *testing.T) {
	actualA := mock{a: 1}
	rand1 := 0
	rand2 := 5.2
	fmt.Println("test address of random variables")
	fmt.Printf("address of actualA %p \n", &actualA)
	fmt.Printf("val of actualA %v \n", &actualA)
	fmt.Println(&rand1)
	fmt.Println(&rand2)
	reconfigA, err := WrapWithReconfigurable(actualA) // this is already a pointer
	test.That(t, err, test.ShouldBeNil)

	fmt.Println("test if allocating to interface changes")
	reconfReconfigA := reconfigA.(*ReconfigurableService) // this is the part that gets fucked up
	//fmt.Println(reconfReconfigA.Actual)
	fmt.Println(&reconfReconfigA)
	fmt.Println(&(reconfReconfigA.Actual))

	reconfReconfigC := reconfigA.(*ReconfigurableService) // this is the part that gets fucked up
	//fmt.Println(reconfReconfigB.Actual)
	fmt.Println(&reconfReconfigC)
	fmt.Println(&(reconfReconfigC.Actual))

	fmt.Println("end test")
	// test.That(t, ok, test.ShouldBeTrue)
	actualActual := reconfReconfigA.Actual // reference to pointer

	fmt.Printf("actualActual addy %p \n", &actualActual)
	fmt.Printf("actualActual val %v \n", actualActual)
	// mockActual, ok := (*actualActual).(Mock)

	// actualAA := *actualActual

	actualB := mock3{num1: 2, num2: 5.2, num3: []int{0, 5, 1, 6, 2, 4}}
	reconfigB, err := WrapWithReconfigurable(actualB)
	test.That(t, err, test.ShouldBeNil)

	reconfReconfigB, _ := reconfigB.(*ReconfigurableService)
	fmt.Println(reconfReconfigB.Actual)
	actualActualB := reconfReconfigB.Actual

	reconfReconfigA.Reconfigure(context.Background(), reconfReconfigB)
	fmt.Println(reconfReconfigA.Actual)
	fmt.Println(*(reconfReconfigA.Actual))
	mockActual, ok := (*actualActual).(mock3)
	if !ok {
		fmt.Println("failed to be mock3")
	}
	mockActualB := (*actualActualB).(mock3)

	test.That(t, mockActual.num1, test.ShouldEqual, mockActualB.num1)
	// test.That(t, actualActual, test.ShouldEqual, actualActualB)

}

type mock struct {
	a int
}

func (m *mock) Get() int {
	return m.a
}

type Mock interface {
	Get() int
}
