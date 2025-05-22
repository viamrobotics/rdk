package webcam

import "fmt"

// Represents image format code used by V4L2 subsystem.
// Number of formats can be different in various
// Linux kernel versions
// See /usr/include/linux/videodev2.h for full list
// of supported image formats
type PixelFormat uint32

// Struct that describes frame size supported by a webcam
// For fixed sizes min and max values will be the same and
// step value will be equal to '0'
type FrameSize struct {
	MinWidth  uint32
	MaxWidth  uint32
	StepWidth uint32

	MinHeight  uint32
	MaxHeight  uint32
	StepHeight uint32
}

// FrameRate represents all possible framerates supported by a webcam
// for a given pixel format and frame size. For discrete returns min
// and max values will be the same and step value will be equal to '0'
// Stepwise returns are represented as a range of values with a step.
type FrameRate struct {
	MinNumerator  uint32
	MaxNumerator  uint32
	StepNumerator uint32

	MinDenominator  uint32
	MaxDenominator  uint32
	StepDenominator uint32
}

// Return string representation of FrameRate struct. e.g.1/30 for
// discrete framerates and [1-2;1]/[10-60;1] for stepwise framerates.
func (f FrameRate) String() string {
	if f.StepNumerator == 0 && f.StepDenominator == 0 {
		return fmt.Sprintf("%d/%d", f.MinNumerator, f.MinDenominator)
	} else {
		return fmt.Sprintf("[%d-%d;%d]/[%d-%d;%d]", f.MinNumerator, f.MaxNumerator, f.StepNumerator, f.MinDenominator, f.MaxDenominator, f.StepDenominator)
	}
}

// Returns string representation of frame size, e.g.
// 1280x720 for fixed-size frames and
// [320-640;160]x[240-480;160] for stepwise-sized frames
func (s FrameSize) GetString() string {
	if s.StepWidth == 0 && s.StepHeight == 0 {
		return fmt.Sprintf("%dx%d", s.MaxWidth, s.MaxHeight)
	} else {
		return fmt.Sprintf("[%d-%d;%d]x[%d-%d;%d]", s.MinWidth, s.MaxWidth, s.StepWidth, s.MinHeight, s.MaxHeight, s.StepHeight)
	}
}
