package datamanager_test

import (
	"bytes"
	"io"
	"os"

	"go.uber.org/multierr"
)

// the function below was taken from:
// https://stackoverflow.com/questions/29505089/how-can-i-compare-two-files-in-golang
const chunkSize = 64000

func deepCompare(file1, file2 string) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	for {
		b1 := make([]byte, chunkSize)
		_, err1 := f1.Read(b1)

		b2 := make([]byte, chunkSize)
		_, err2 := f2.Read(b2)

		if err1 != nil || err2 != nil {
			switch {
			case err1 == io.EOF && err2 == io.EOF:
				return true, nil
			case err1 == io.EOF || err2 == io.EOF:
				return false, nil
			}
			return false, multierr.Combine(err1, err2)
		}

		if !bytes.Equal(b1, b2) {
			return false, err
		}
	}
}
