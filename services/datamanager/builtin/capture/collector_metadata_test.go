package capture

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/services/datamanager"
)

func TestCollectorMetadata(t *testing.T) {
	t.Run("newCollectorMetadata()", func(t *testing.T) {
		arm := arm.Named("arm1")
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
			Name:             arm,
			Method:           "JointPositions",
			AdditionalParams: map[string]string{"some": "input"},
		})
		test.That(t, md, test.ShouldResemble, collectorMetadata{
			ResourceName: "arm1",
			MethodParams: "map[some:input]",
			MethodMetadata: data.MethodMetadata{
				API:        arm.API,
				MethodName: "JointPositions",
			},
		})
	})

	t.Run("String()", func(t *testing.T) {
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
			Name:             arm.Named("arm1"),
			Method:           "JointPositions",
			AdditionalParams: map[string]string{"some": "input"},
		})
		exp := "[Resource Name: arm1, API: rdk:component:arm, Method Name: JointPositions, Method Params: map[some:input]]"
		test.That(t, md.String(), test.ShouldResemble, exp)
	})
}
