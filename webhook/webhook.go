package webhook

// Define a generic function type that takes two arguments of the same type T and returns a bool.
type Compare[T comparable] func(a, b T) bool

type DataCaptureWebhook[T comparable] struct {
	Comparator Compare[T]
	URL        string
}

func NewDataCaptureWebhook[T comparable](c *WebhookConfig) DataCaptureWebhook[T] {
	if c == nil {
		// An "empty" data capture webhook does not pass a webhook url through to
		// the data capture flow (and thus no webhooks will be emitted).
		return DataCaptureWebhook[T]{Comparator: func(a, b T) bool { return false }}
	}
	var fn Compare[T]
	switch c.Comparator {
	case "eq":
		fn = func(a, b T) bool {
			return a == b
		}
	case "neq":
		fn = func(a, b T) bool {
			return a != b
		}
	default:
		// Could not configure the data capture webhook properly, so return a
		// data capture webhook that is essentially a no-op.
		return DataCaptureWebhook[T]{Comparator: func(a, b T) bool { return false }}
	}
	return DataCaptureWebhook[T]{
		Comparator: fn,
		URL:        (c.Emit)["url"][0],
	}
}

func (dcw *DataCaptureWebhook[T]) Compare(a, b T) string {
	if dcw.Comparator(a, b) {
		return dcw.URL
	}
	return ""
}

type WebhookConfig struct {
	Comparator string                `json:"comparator"` // should not be omitempty, we need this!
	Value      any                   `json:"value"`      // should not be omitempty, we need this!
	Emit       map[string]([]string) `json:"emit"`       // should not be omitempty, we need this!
}
