package functionvm

// EngineName is the name of an engine, typically bound to a single
// programming language.
type EngineName string

// The known engine names.
const (
	EngineNameJavaScript = EngineName("javascript")
)

// An Engine is responsible for executing code in its target language.
type Engine interface {
	// ExecuteSource evaluates the given source.
	ExecuteSource(source string) ([]Value, error)

	// ValidateSource ensures the given source can compile.
	ValidateSource(source string) error

	// ImportFunction injects the given function into the engine.
	// Note(erd): This may not be universally possible, so this may need to
	// be a specialized interface or moved.
	ImportFunction(name string, f Function) error

	// StandardOutput returns the output so far from the standard out stream.
	StandardOutput() string

	// StandardError returns the output so far from the standard error stream.
	StandardError() string
}
