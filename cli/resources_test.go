package cli

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
)

func TestResourceListing(t *testing.T) {
	names := []resource.Name{
		vision.Named("segmenter"),
		camera.Named("wrist-cam"),
		arm.Named("pick-arm"),
		resource.NewName(resource.APINamespaceRDKInternal.WithServiceType("frame_system"), "$frame_system"),
	}
	listed := map[string][]string{
		"viam.component.arm.v1.ArmService": {
			"viam.component.arm.v1.ArmService.GetJointPositions",
			"viam.component.arm.v1.ArmService.MoveToPosition",
		},
		"viam.component.camera.v1.CameraService": {"viam.component.camera.v1.CameraService.GetImages"},
	}
	lines := resourceListing(names, func(service string) []string { return listed[service] })

	test.That(t, len(lines), test.ShouldEqual, 3) // the internal $frame_system resource is omitted
	// Ordered by API then name: arm, camera, vision.
	test.That(t, lines[0], test.ShouldContainSubstring, "pick-arm")
	test.That(t, lines[0], test.ShouldContainSubstring, "rdk:component:arm")
	test.That(t, lines[0], test.ShouldContainSubstring, "GetJointPositions, MoveToPosition")
	test.That(t, lines[0], test.ShouldNotContainSubstring, "viam.component.arm.v1")
	test.That(t, lines[1], test.ShouldContainSubstring, "wrist-cam")
	test.That(t, lines[1], test.ShouldContainSubstring, "GetImages")
	// A service reflection does not report still lists the resource, with a hint.
	test.That(t, lines[2], test.ShouldContainSubstring, "segmenter")
	test.That(t, lines[2], test.ShouldContainSubstring, "methods not reported")
}

func TestAPIToGRPCServiceNameForListing(t *testing.T) {
	test.That(t, apiToGRPCServiceName(arm.API), test.ShouldEqual, "viam.component.arm.v1.ArmService")
	test.That(t, apiToGRPCServiceName(vision.API), test.ShouldEqual, "viam.service.vision.v1.VisionService")
}
