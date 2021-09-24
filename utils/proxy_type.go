package utils

// A ProxyType is a type that proxies behavior on behalf of some other type.
// This uses empty interfaces in lieu of generics existing and not wanting
// to duplicate much code. Ideally this never needs to be used but in cases
// where a concrete type must be reached and it's interface cannot be used,
// this is useful.
type ProxyType interface {
	ProxyFor() interface{}
}

// UnwrapProxy unwraps a proxy as far as possible.
func UnwrapProxy(v interface{}) interface{} {
	if pt, ok := v.(ProxyType); ok {
		return UnwrapProxy(pt.ProxyFor())
	}
	return v
}
