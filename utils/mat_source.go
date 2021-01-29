package utils

import (
	"gocv.io/x/gocv"
)

type MatSource interface {
	NextMat() (gocv.Mat, error)
	Close()
}
