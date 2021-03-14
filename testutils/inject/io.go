package inject

import "io"

type ReadWriteCloser struct {
	io.ReadWriteCloser
	ReadFunc  func(p []byte) (n int, err error)
	WriteFunc func(p []byte) (n int, err error)
	CloseFunc func() error
}

func (rwc *ReadWriteCloser) Read(p []byte) (n int, err error) {
	if rwc.ReadFunc == nil {
		return rwc.ReadWriteCloser.Read(p)
	}
	return rwc.ReadFunc(p)
}

func (rwc *ReadWriteCloser) Write(p []byte) (n int, err error) {
	if rwc.WriteFunc == nil {
		return rwc.ReadWriteCloser.Write(p)
	}
	return rwc.WriteFunc(p)
}

func (rwc *ReadWriteCloser) Close() error {
	if rwc.CloseFunc == nil {
		return rwc.ReadWriteCloser.Close()
	}
	return rwc.CloseFunc()
}
