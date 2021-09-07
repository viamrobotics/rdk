package functionvm

// EngineName is the name of an engine, typically bound to a single
// programming language.
type EngineName string

// The known engine names.
const (
	EngineNameJavaScript = EngineName("javascript")
)

// An Engine is responsible for execution code in its target language.
type Engine interface {
	ExecuteCode(code string) ([]Value, error)
}
