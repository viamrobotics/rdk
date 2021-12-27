package functionvm_test

import (
	"errors"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	functionvm "go.viam.com/rdk/function/vm"
	"go.viam.com/rdk/testutils/inject"
)

func TestRegistryRegisterEngine(t *testing.T) {
	engName1 := utils.RandomAlphaString(64)
	engName2 := utils.RandomAlphaString(64)
	functionvm.RegisterEngine(functionvm.EngineName(engName1), func() (functionvm.Engine, error) {
		return nil, nil
	})
	functionvm.RegisterEngine(functionvm.EngineName(engName2), func() (functionvm.Engine, error) {
		return nil, nil
	})
	test.That(t, func() {
		functionvm.RegisterEngine(functionvm.EngineName(engName1), func() (functionvm.Engine, error) {
			return nil, nil
		})
	}, test.ShouldPanic)
}

func TestRegistryNewEngine(t *testing.T) {
	_, err := functionvm.NewEngine(functionvm.EngineName("unknown"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no engine")

	engName1 := utils.RandomAlphaString(64)
	engName2 := utils.RandomAlphaString(64)
	engName3 := utils.RandomAlphaString(64)
	injectEngine1 := &inject.Engine{}
	injectEngine2 := &inject.Engine{}
	functionvm.RegisterEngine(functionvm.EngineName(engName1), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})
	functionvm.RegisterEngine(functionvm.EngineName(engName2), func() (functionvm.Engine, error) {
		return injectEngine2, nil
	})
	functionvm.RegisterEngine(functionvm.EngineName(engName3), func() (functionvm.Engine, error) {
		return nil, errors.New("whoops")
	})
	eng1, err := functionvm.NewEngine(functionvm.EngineName(engName1))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eng1, test.ShouldEqual, injectEngine1)
	eng2, err := functionvm.NewEngine(functionvm.EngineName(engName2))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eng2, test.ShouldEqual, injectEngine2)
	_, err = functionvm.NewEngine(functionvm.EngineName(engName3))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
}

func TestRegistryValidateSource(t *testing.T) {
	err := functionvm.ValidateSource(functionvm.EngineName("unknown"), "1+")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no engine")

	engName1 := utils.RandomAlphaString(64)
	injectEngine1 := &inject.Engine{}
	functionvm.RegisterEngine(functionvm.EngineName(engName1), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})

	injectEngine1.ValidateSourceFunc = func(_ string) error {
		return errors.New("whoops")
	}
	err = functionvm.ValidateSource(functionvm.EngineName(engName1), "1+")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	injectEngine1.ValidateSourceFunc = func(_ string) error {
		return nil
	}
	err = functionvm.ValidateSource(functionvm.EngineName(engName1), "1+")
	test.That(t, err, test.ShouldBeNil)
}
