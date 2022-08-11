package resource

// DefaultServices is a list of default robot services.
// services should add themseleves in an init if they should be included by default.
var DefaultServices []Name

// AddDefaultService add a default service.
func AddDefaultService(n Name) {
	DefaultServices = append(DefaultServices, n)
}
