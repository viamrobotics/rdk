package inject

import "io"

// ReadWriteCloser is an injected read write closer.
type ReadWriteCloser struct {
	io.ReadWriteCloser
	ReadFunc  func(p []byte) (n int, err error)
	WriteFunc func(p []byte) (n int, err error)
	CloseFunc func() error
}

// Read calls the injected Read or the real version.
func (rwc *ReadWriteCloser) Read(p []byte) (n int, err error) {
	if rwc.ReadFunc == nil {
		return rwc.ReadWriteCloser.Read(p)
	}
	return rwc.ReadFunc(p)
}

// Write calls the injected Write or the real version.
func (rwc *ReadWriteCloser) Write(p []byte) (n int, err error) {
	if rwc.WriteFunc == nil {
		return rwc.ReadWriteCloser.Write(p)
	}
	return rwc.WriteFunc(p)
}

// Close calls the injected Close or the real version.
func (rwc *ReadWriteCloser) Close() error {
	if rwc.CloseFunc == nil {
		return rwc.ReadWriteCloser.Close()
	}
	return rwc.CloseFunc()
}
