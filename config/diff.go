package config

import (
	"encoding/json"
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
	Modified    *Config
	Removed     *Config
	Equal       bool
	prettyDiff  string
}

// DiffConfigs returns the difference between the two given configs
// from left to right.
func DiffConfigs(left, right *Config) (*Diff, error) {
	prettyDiff, err := prettyDiff(left, right)
	if err != nil {
		return nil, err
	}

	diff := Diff{
		Left:       left,
		Right:      right,
		Added:      &Config{},
		Modified:   &Config{},
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
	different = diffBoards(left.Boards, right.Boards, &diff) || different
	different = diffComponents(left.Components, right.Components, &diff) || different
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

func diffBoards(left, right []board.Config, diff *Diff) bool {
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
			different = diffBoard(l, r, diff) || different
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
	return different
}

// TODO(https://github.com/viamrobotics/robotcore/issues/44): diff deeper
func diffBoard(left, right board.Config, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Boards = append(diff.Modified.Boards, right)
	return true
}

func diffComponents(left, right []Component, diff *Diff) bool {
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
			different = diffComponent(l, r, diff) || different
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
	return different
}

func diffComponent(left, right Component, diff *Diff) bool {
	if reflect.DeepEqual(left, right) {
		return false
	}
	diff.Modified.Components = append(diff.Modified.Components, right)
	return true
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
