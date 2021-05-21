package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"go.viam.com/core/board"
	"go.viam.com/core/rexec"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// A Diff is the difference between two configs, left and right
// where left is usually old and right is new. So the diff is the
// changes from left to right.
type Diff struct {
	Left, Right *Config
	Added       *Config
	Modified    *ModifiedConfigDiff
	Removed     *Config
	Equal       bool
	prettyDiff  string
}

// ModifiedConfigDiff is the modificative different between two configs.
type ModifiedConfigDiff struct {
	Remotes    []Remote
	Boards     map[string]board.ConfigDiff
	Components []Component
	Processes  []rexec.ProcessConfig
}

// DiffConfigs returns the difference between the two given configs
// from left to right.
func DiffConfigs(left, right *Config) (*Diff, error) {
	prettyDiff, err := prettyDiff(left, right)
	if err != nil {
		return nil, err
	}

	diff := Diff{
		Left:  left,
		Right: right,
		Added: &Config{},
		Modified: &ModifiedConfigDiff{
			Boards: map[string]board.ConfigDiff{},
		},
		Removed:    &Config{},
		prettyDiff: prettyDiff,
	}

	// All diffs use the following logic:
	// If left contains something right does not => removed
	// If right contains something left does not => added
	// If left contains something right does and they are not equal => modified
	// If left contains something right does and they are equal => no diff
	// Note: generics would be nice here!
	different := diffRemotes(left.Remotes, right.Remotes, &diff)
	boardsDifferent, err := diffBoards(left.Boards, right.Boards, &diff)
	if err != nil {
		return nil, err
	}
	different = boardsDifferent || different
	componentsDifferent, err := diffComponents(left.Components, right.Components, &diff)
	if err != nil {
		return nil, err
	}
	different = componentsDifferent || different
	different = diffProcesses(left.Processes, right.Processes, &diff) || different
	diff.Equal = !different

	return &diff, nil
}

func prettyDiff(left, right *Config) (string, error) {
	leftMd, err := json.MarshalIndent(left, "", " ")
	if err != nil {
		return "", err
	}
	rightMd, err := json.MarshalIndent(right, "", " ")
	if err != nil {
		return "", err
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(leftMd), string(rightMd), false)
	filteredDiffs := make([]diffmatchpatch.Diff, 0, len(diffs))
	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		filteredDiffs = append(filteredDiffs, d)
	}
	return dmp.DiffPrettyText(filteredDiffs), nil
}

// String returns a pretty version of the diff.
func (diff *Diff) String() string {
	return diff.prettyDiff
}

func diffRemotes(left, right []Remote, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Remote)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffRemote(l, r, diff) || different
			continue
		}
		diff.Added.Remotes = append(diff.Added.Remotes, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Remotes = append(diff.Removed.Remotes, left[idx])
	}
	return different
}

func diffRemote(left, right Remote, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Remotes = append(diff.Modified.Remotes, right)
	return true
}

func diffBoards(left, right []board.Config, diff *Diff) (bool, error) {
	leftIndex := make(map[string]int)
	leftM := make(map[string]board.Config)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			boardDifferent, err := diffBoard(l, r, diff)
			if err != nil {
				return false, err
			}
			different = boardDifferent || different
			continue
		}
		diff.Added.Boards = append(diff.Added.Boards, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Boards = append(diff.Removed.Boards, left[idx])
	}
	return different, nil
}

func diffBoard(left, right board.Config, diff *Diff) (bool, error) {
	if reflect.DeepEqual(left, right) {
		return false, nil
	}

	boardDiff := board.ConfigDiff{
		Left:  &left,
		Right: &right,
		Added: &board.Config{
			Name:  right.Name,
			Model: right.Model,
		},
		Modified: &board.Config{
			Name:  right.Name,
			Model: right.Model,
		},
		Removed: &board.Config{
			Name:  right.Name,
			Model: right.Model,
		},
	}

	different := diffBoardMotors(left.Motors, right.Motors, &boardDiff)
	different = diffBoardServos(left.Servos, right.Servos, &boardDiff) || different
	different = diffBoardAnalogs(left.Analogs, right.Analogs, &boardDiff) || different
	interruptsDifferent, err := diffBoardDigitalInterrupts(left.DigitalInterrupts, right.DigitalInterrupts, &boardDiff)
	if err != nil {
		return false, err
	}
	different = interruptsDifferent || different

	if !different {
		return false, nil
	}

	diff.Modified.Boards[right.Name] = boardDiff
	return true, nil
}

func diffBoardMotors(left, right []board.MotorConfig, diff *board.ConfigDiff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]board.MotorConfig)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffBoardMotor(l, r, diff) || different
			continue
		}
		diff.Added.Motors = append(diff.Added.Motors, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Motors = append(diff.Removed.Motors, left[idx])
	}
	return different
}

func diffBoardMotor(left, right board.MotorConfig, diff *board.ConfigDiff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Motors = append(diff.Modified.Motors, right)
	return true
}

func diffBoardServos(left, right []board.ServoConfig, diff *board.ConfigDiff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]board.ServoConfig)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffBoardServo(l, r, diff) || different
			continue
		}
		diff.Added.Servos = append(diff.Added.Servos, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Servos = append(diff.Removed.Servos, left[idx])
	}
	return different
}

func diffBoardServo(left, right board.ServoConfig, diff *board.ConfigDiff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Servos = append(diff.Modified.Servos, right)
	return true
}

func diffBoardAnalogs(left, right []board.AnalogConfig, diff *board.ConfigDiff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]board.AnalogConfig)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			different = diffBoardAnalog(l, r, diff) || different
			continue
		}
		diff.Added.Analogs = append(diff.Added.Analogs, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Analogs = append(diff.Removed.Analogs, left[idx])
	}
	return different
}

func diffBoardAnalog(left, right board.AnalogConfig, diff *board.ConfigDiff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Analogs = append(diff.Modified.Analogs, right)
	return true
}

func diffBoardDigitalInterrupts(left, right []board.DigitalInterruptConfig, diff *board.ConfigDiff) (bool, error) {
	leftIndex := make(map[string]int)
	leftM := make(map[string]board.DigitalInterruptConfig)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			interruptDiffrent, err := diffBoardDigitalInterrupt(l, r, diff)
			if err != nil {
				return false, err
			}
			different = interruptDiffrent || different
			continue
		}
		diff.Added.DigitalInterrupts = append(diff.Added.DigitalInterrupts, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.DigitalInterrupts = append(diff.Removed.DigitalInterrupts, left[idx])
	}
	return different, nil
}

func diffBoardDigitalInterrupt(
	left, right board.DigitalInterruptConfig, diff *board.ConfigDiff) (bool, error) {
	if reflect.DeepEqual(left, right) {
		return false, nil
	}
	diff.Modified.DigitalInterrupts = append(diff.Modified.DigitalInterrupts, right)
	return true, nil
}

func diffComponents(left, right []Component, diff *Diff) (bool, error) {
	leftIndex := make(map[string]int)
	leftM := make(map[string]Component)
	for idx, l := range left {
		leftM[l.Name] = l
		leftIndex[l.Name] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.Name]
		delete(leftM, r.Name)
		if ok {
			componentDifferent, err := diffComponent(l, r, diff)
			if err != nil {
				return false, err
			}
			different = componentDifferent || different
			continue
		}
		diff.Added.Components = append(diff.Added.Components, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Components = append(diff.Removed.Components, left[idx])
	}
	return different, nil
}

func diffComponent(left, right Component, diff *Diff) (bool, error) {
	if reflect.DeepEqual(left, right) {
		return false, nil
	}
	if left.Type != right.Type || left.SubType != right.SubType {
		return false, fmt.Errorf("cannot modify type of existing component %q", left.Name)
	}
	diff.Modified.Components = append(diff.Modified.Components, right)
	return true, nil
}

func diffProcesses(left, right []rexec.ProcessConfig, diff *Diff) bool {
	leftIndex := make(map[string]int)
	leftM := make(map[string]rexec.ProcessConfig)
	for idx, l := range left {
		leftM[l.ID] = l
		leftIndex[l.ID] = idx
	}

	var removed []int

	var different bool
	for _, r := range right {
		l, ok := leftM[r.ID]
		delete(leftM, r.ID)
		if ok {
			different = diffProcess(l, r, diff) || different
			continue
		}
		diff.Added.Processes = append(diff.Added.Processes, r)
		different = true
	}

	for k := range leftM {
		removed = append(removed, leftIndex[k])
		different = true
	}
	sort.Ints(removed)
	for _, idx := range removed {
		diff.Removed.Processes = append(diff.Removed.Processes, left[idx])
	}
	return different
}

func diffProcess(left, right rexec.ProcessConfig, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Processes = append(diff.Modified.Processes, right)
	return true
}
