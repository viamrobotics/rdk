package resource

// the default field that API calls expect is "name".
const defaultResourceField = "name"

// resourceNameOverrides is a map for an edge case handling of certain APIs. In
// particular, the "inputcontroller" API expects a different argument
// ("controller", rather than "name") for this set of functions.
var resourceNameOverrides = map[string]map[string]string{
	"viam.component.inputcontroller.v1.InputControllerService": {
		"GetControls":   "controller",
		"GetEvents":     "controller",
		"StreamEvents":  "controller",
		"TriggerEvents": "controller",
	},
}

// GetResourceNameOverride checks if the provided service and its method need special
// handling based on the resourceNameOverrides map. Returns what should be the "resource
// name" for this particular gRPC request.
func GetResourceNameOverride(service, method string) string {
	if mapService := resourceNameOverrides[service]; mapService != nil {
		if resourceName := mapService[method]; resourceName != "" {
			return resourceName
		}
	}
	return defaultResourceField
}
