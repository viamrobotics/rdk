package test

import (
	"fmt"

	assertions "github.com/dgottlieb/smarty-assertions"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	success                = ""
	needExactValues        = "This assertion requires exactly %d comparison values (you provided %d)."
	shouldHaveResembled    = "Expected: '%s'\nActual:   '%s'\n(Should resemble)!"
	shouldNotHaveResembled = "Expected        '%#v'\nto NOT resemble '%#v'\n(but it did)!"
)

func need(needed int, expected []interface{}) string {
	if len(expected) != needed {
		return fmt.Sprintf(needExactValues, needed, len(expected))
	}
	return success
}

// ShouldResembleProto receives exactly two parameters and does a proto equal check.
func ShouldResembleProto(actual interface{}, expected ...interface{}) string {
	if message := need(1, expected); message != success {
		return message
	}

	if cmp.Equal(actual, expected[0], protocmp.Transform()) {
		return ""
	}

	return fmt.Sprintf(shouldHaveResembled, actual, expected[0]) +
		cmp.Diff(actual, expected[0], protocmp.Transform())
}

// ShouldNotResembleProto receives exactly two parameters and does an inverse proto equal check.
func ShouldNotResembleProto(actual interface{}, expected ...interface{}) string {
	if message := need(1, expected); message != success {
		return message
	} else if ShouldResembleProto(actual, expected[0]) == success {
		return fmt.Sprintf(shouldNotHaveResembled, actual, expected[0])
	}
	return success
}

// ShouldResemble receives exactly two parameters and does a deep equal check (see reflect.DeepEqual)
func ShouldResemble(actual interface{}, expected ...interface{}) string {
	if message := need(1, expected); message != success {
		return message
	}

	_, actualIsProto := actual.(proto.Message)
	_, expectedIsProto := expected[0].(proto.Message)
	if actualIsProto && expectedIsProto {
		return ShouldResembleProto(actual, expected...)
	}

	return assertions.ShouldResemble(actual, expected...)
}

// ShouldNotResemble receives exactly two parameters and does an inverse deep equal check (see reflect.DeepEqual)
func ShouldNotResemble(actual interface{}, expected ...interface{}) string {
	if message := need(1, expected); message != success {
		return message
	}

	_, actualIsProto := actual.(proto.Message)
	_, expectedIsProto := expected[0].(proto.Message)
	if actualIsProto && expectedIsProto {
		return ShouldNotResembleProto(actual, expected...)
	}

	return assertions.ShouldNotResemble(actual, expected...)
}
