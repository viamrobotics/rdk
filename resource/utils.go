package resource

type resourceNamer struct {
	nameField       string
	nameFromMessage func(any) string
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
	controllerServiceResourceNamer = resourceNamer{
		nameField: "controller",
		nameFromMessage: func(msg any) string {
			if namer, ok := msg.(interface{ GetController() string }); ok {
				return namer.GetController()
			}
			return ""
		},
	}
)

// resourceNameOverrides is a map for an edge case handling of certain APIs. In
// particular, the "inputcontroller" API expects a different argument
// ("controller", rather than "name") for this set of functions.
var resourceNameOverrides = map[string]map[string]*resourceNamer{
	"viam.component.inputcontroller.v1.InputControllerService": {
		"GetControls":   &controllerServiceResourceNamer,
		"GetEvents":     &controllerServiceResourceNamer,
		"StreamEvents":  &controllerServiceResourceNamer,
		"TriggerEvents": &controllerServiceResourceNamer,
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
