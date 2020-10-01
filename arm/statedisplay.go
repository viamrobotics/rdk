package arm

import (
	"fmt"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

type StateDisplay struct {
	xLabel *widget.Label
	yLabel *widget.Label
	zLabel *widget.Label

	TheContainer *fyne.Container
}

func NewStateDisplay() *StateDisplay {
	s := &StateDisplay{}
	s.xLabel = widget.NewLabel("5")
	s.yLabel = widget.NewLabel("17")
	s.zLabel = widget.NewLabel("81")

	myLayout := layout.NewGridLayout(2)

	s.TheContainer = fyne.NewContainerWithLayout(
		myLayout,
		widget.NewLabel("X"),
		s.xLabel,
		widget.NewLabel("Y"),
		s.yLabel,
		widget.NewLabel("Z"),
		s.zLabel,
	)

	return s
}

func (sd *StateDisplay) Update(state RobotState) {
	sd.xLabel.SetText(fmt.Sprintf("%f", state.CartesianInfo.X))
	sd.yLabel.SetText(fmt.Sprintf("%f", state.CartesianInfo.Y))
	sd.zLabel.SetText(fmt.Sprintf("%f", state.CartesianInfo.Z))
}
