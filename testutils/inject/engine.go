package inject

import functionvm "go.viam.com/core/function/vm"

// Engine is an injected engine.
type Engine struct {
	functionvm.Engine
	ExecuteSourceFunc  func(source string) ([]functionvm.Value, error)
	ValidateSourceFunc func(source string) error
	ImportFunctionFunc func(name string, f functionvm.Function) error
	StandardOutputFunc func() string
	StandardErrorFunc  func() string
}

// ExecuteSource calls the injected ExecuteSource or the real version.
func (e *Engine) ExecuteSource(source string) ([]functionvm.Value, error) {
	if e.ExecuteSourceFunc == nil {
		return e.Engine.ExecuteSource(source)
	}
	return e.ExecuteSourceFunc(source)
}

// ValidateSource calls the injected ValidateSource or the real version.
func (e *Engine) ValidateSource(source string) error {
	if e.ValidateSourceFunc == nil {
		return e.Engine.ValidateSource(source)
	}
	return e.ValidateSourceFunc(source)
}

// ImportFunction calls the injected ImportFunction or the real version.
func (e *Engine) ImportFunction(name string, f functionvm.Function) error {
	if e.ImportFunctionFunc == nil {
		return e.Engine.ImportFunction(name, f)
	}
	return e.ImportFunctionFunc(name, f)
}

// StandardOutput calls the injected StandardOutput or the real version.
func (e *Engine) StandardOutput() string {
	if e.StandardOutputFunc == nil {
		return e.Engine.StandardOutput()
	}
	return e.StandardOutputFunc()
}

// StandardError calls the injected StandardError or the real version.
func (e *Engine) StandardError() string {
	if e.StandardErrorFunc == nil {
		return e.Engine.StandardError()
	}
	return e.StandardErrorFunc()
}
