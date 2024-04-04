package webhook

// Define a generic function type that takes two arguments of the same type T and returns a bool.
type Compare[T comparable] func(a, b T) bool

type DataCaptureWebhook[T comparable] struct {
	Comparator Compare[T]
	URL        string
}

func NewDataCaptureWebhook[T comparable](url string) *DataCaptureWebhook[T] {
	return &DataCaptureWebhook[T]{
		Comparator: func(a, b T) bool { return a == b },
		URL:        url,
	}
}
