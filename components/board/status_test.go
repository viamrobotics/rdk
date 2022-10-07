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
		map[string]interface{}{
			"analogs":            map[string]interface{}{"analog1": map[string]interface{}{}},
			"digital_interrupts": map[string]interface{}{"encoder": map[string]interface{}{}},
		},
	)

	convMap := &commonpb.BoardStatus{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}
