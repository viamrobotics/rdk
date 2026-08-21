package resource

import "reflect"

type resourceNamer struct {
	nameField       string
	nameFromMessage func(any) string
}

// nestedNameFromMessage reads a resource name reached by calling a chain of zero-arg,
// single-return getters on the message, e.g. GetMetadata().GetName(). It is used for
// streaming endpoints that carry the resource name inside a oneof in their first
// message. It returns "" if any getter is missing or an intermediate value is nil.
func nestedNameFromMessage(getters ...string) func(any) string {
	return func(msg any) string {
		v := reflect.ValueOf(msg)
		for _, getter := range getters {
			if !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil()) {
				return ""
			}
			method := v.MethodByName(getter)
			if !method.IsValid() || method.Type().NumIn() != 0 || method.Type().NumOut() != 1 {
				return ""
			}
			v = method.Call(nil)[0]
		}
		if v.Kind() == reflect.String {
			return v.String()
		}
		return ""
	}
}

// the default field that API calls expect is "name".
var (
	defaultResourceNamer = resourceNamer{
		nameField: "name",
		nameFromMessage: func(msg any) string {
			if namer, ok := msg.(interface{ GetName() string }); ok {
				return namer.GetName()
			}
			return ""
		},
	}
	// The inputcontroller API expects "controller" rather than "name".
	controllerServiceResourceNamer = resourceNamer{
		nameField: "controller",
		nameFromMessage: func(msg any) string {
			if namer, ok := msg.(interface{ GetController() string }); ok {
				return namer.GetController()
			}
			return ""
		},
	}
	// Several board API calls address the board via "board_name" (their "name" field, if
	// any, identifies a sub-component such as an analog reader or digital interrupt).
	boardServiceResourceNamer = resourceNamer{
		nameField: "board_name",
		nameFromMessage: func(msg any) string {
			if namer, ok := msg.(interface{ GetBoardName() string }); ok {
				return namer.GetBoardName()
			}
			return ""
		},
	}
	// audioout PlayStream is client-streaming; the resource name is the "name" field of
	// the PlayStreamInit carried in the first message's oneof.
	audioOutServiceResourceNamer = resourceNamer{
		nameField:       "name",
		nameFromMessage: nestedNameFromMessage("GetInit", "GetName"),
	}
	// shell CopyFiles{To,From}Machine are streaming; the resource name is the "name"
	// field of the metadata carried in the first message's oneof.
	shellCopyFilesResourceNamer = resourceNamer{
		nameField:       "name",
		nameFromMessage: nestedNameFromMessage("GetMetadata", "GetName"),
	}
)

// resourceNameOverrides handles APIs whose gRPC requests do not carry the resource name
// under a top-level "name" field. Without an entry here, such a request resolves to no
// resource name and (for authorization) falls under machine-wide "_machine" grants
// rather than being scoped to the resource it actually addresses.
var resourceNameOverrides = map[string]map[string]*resourceNamer{
	"viam.component.inputcontroller.v1.InputControllerService": {
		"GetControls":  &controllerServiceResourceNamer,
		"GetEvents":    &controllerServiceResourceNamer,
		"StreamEvents": &controllerServiceResourceNamer,
		"TriggerEvent": &controllerServiceResourceNamer,
	},
	"viam.component.board.v1.BoardService": {
		"ReadAnalogReader":         &boardServiceResourceNamer,
		"GetDigitalInterruptValue": &boardServiceResourceNamer,
	},
	"viam.component.audioout.v1.AudioOutService": {
		"PlayStream": &audioOutServiceResourceNamer,
	},
	"viam.service.shell.v1.ShellService": {
		"CopyFilesToMachine":   &shellCopyFilesResourceNamer,
		"CopyFilesFromMachine": &shellCopyFilesResourceNamer,
	},
}

func getResourceNamer(service, method string) *resourceNamer {
	if mapService := resourceNameOverrides[service]; mapService != nil {
		if rn := mapService[method]; rn != nil {
			return rn
		}
	}
	return &defaultResourceNamer
}

// GetResourceNameOverride checks if the provided service and its method need special
// handling based on the resourceNameOverrides map. Returns what should be the "resource
// name" for this particular gRPC request.
func GetResourceNameOverride(service, method string) string {
	return getResourceNamer(service, method).nameField
}

// GetResourceNameFromRequest attempts to extract the name of a resource from a
// gRPC request. It returns the name if found or the empty string otherwise.
func GetResourceNameFromRequest(service, method string, req any) string {
	return getResourceNamer(service, method).nameFromMessage(req)
}
