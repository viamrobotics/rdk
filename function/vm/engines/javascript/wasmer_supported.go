//go:build !arm

// Package javascript will load the wasmer js engine if supported
package javascript

import _ "go.viam.com/rdk/function/vm/engines/javascript/impl" // import wasmer if supported.
