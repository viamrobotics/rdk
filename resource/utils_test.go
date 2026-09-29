package resource

import (
	"testing"

	audiooutpb "go.viam.com/api/component/audioout/v1"
	boardpb "go.viam.com/api/component/board/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	inputcontrollerpb "go.viam.com/api/component/inputcontroller/v1"
	shellpb "go.viam.com/api/service/shell/v1"
	"go.viam.com/test"
)

func TestGetResourceNameFromRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		service string
		method  string
		req     any
		want    string
	}{
		{
			"default name field",
			"viam.component.camera.v1.CameraService", "GetImages",
			&camerapb.GetImagesRequest{Name: "cam1"}, "cam1",
		},
		{
			"inputcontroller uses controller field",
			"viam.component.inputcontroller.v1.InputControllerService", "TriggerEvent",
			&inputcontrollerpb.TriggerEventRequest{Controller: "stick"}, "stick",
		},
		{
			"board ReadAnalogReader uses board_name",
			"viam.component.board.v1.BoardService", "ReadAnalogReader",
			&boardpb.ReadAnalogReaderRequest{BoardName: "board1", AnalogReaderName: "a1"}, "board1",
		},
		{
			"board GetDigitalInterruptValue uses board_name",
			"viam.component.board.v1.BoardService", "GetDigitalInterruptValue",
			&boardpb.GetDigitalInterruptValueRequest{BoardName: "board1", DigitalInterruptName: "d1"}, "board1",
		},
		{
			"audioout PlayStream reads name from the init oneof",
			"viam.component.audioout.v1.AudioOutService", "PlayStream",
			&audiooutpb.PlayStreamRequest{Payload: &audiooutpb.PlayStreamRequest_Init{
				Init: &audiooutpb.PlayStreamInit{Name: "speaker"},
			}}, "speaker",
		},
		{
			"shell CopyFilesToMachine reads name from the metadata oneof",
			"viam.service.shell.v1.ShellService", "CopyFilesToMachine",
			&shellpb.CopyFilesToMachineRequest{Request: &shellpb.CopyFilesToMachineRequest_Metadata{
				Metadata: &shellpb.CopyFilesToMachineRequestMetadata{Name: "shell1"},
			}}, "shell1",
		},
		{
			"shell CopyFilesFromMachine reads name from the metadata oneof",
			"viam.service.shell.v1.ShellService", "CopyFilesFromMachine",
			&shellpb.CopyFilesFromMachineRequest{Request: &shellpb.CopyFilesFromMachineRequest_Metadata{
				Metadata: &shellpb.CopyFilesFromMachineRequestMetadata{Name: "shell1"},
			}}, "shell1",
		},
		{
			// a non-metadata first message (e.g. file data) has no name
			"shell CopyFiles data message has no name",
			"viam.service.shell.v1.ShellService", "CopyFilesToMachine",
			&shellpb.CopyFilesToMachineRequest{Request: &shellpb.CopyFilesToMachineRequest_FileData{}}, "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, GetResourceNameFromRequest(tc.service, tc.method, tc.req), test.ShouldEqual, tc.want)
		})
	}
}
