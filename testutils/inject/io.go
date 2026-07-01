package inject

import (
	"braces.dev/errtrace"
	"io"
)

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
		return errtrace.Wrap2(rwc.ReadWriteCloser.Read(p))
	}
	return errtrace.Wrap2(rwc.ReadFunc(p))
}

// Write calls the injected Write or the real version.
func (rwc *ReadWriteCloser) Write(p []byte) (n int, err error) {
	if rwc.WriteFunc == nil {
		return errtrace.Wrap2(rwc.ReadWriteCloser.Write(p))
	}
	return errtrace.Wrap2(rwc.WriteFunc(p))
}

// Close calls the injected Close or the real version.
func (rwc *ReadWriteCloser) Close() error {
	if rwc.CloseFunc == nil {
		return errtrace.Wrap(rwc.ReadWriteCloser.Close())
	}
	return errtrace.Wrap(rwc.CloseFunc())
}
