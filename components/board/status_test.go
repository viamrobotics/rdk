package board_test

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
)

func TestStatusValid(t *testing.T) {
	status := &commonpb.BoardStatus{
		Analogs:           map[string]*commonpb.AnalogStatus{"analog1": {}},
		DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{"encoder": {}},
	}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]any{
			"analogs":            map[string]any{"analog1": map[string]any{}},
			"digital_interrupts": map[string]any{"encoder": map[string]any{}},
		},
	)

	convMap := &commonpb.BoardStatus{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}
